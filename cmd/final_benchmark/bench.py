import json
import math
import matplotlib.pyplot as plt
import numpy as np

with open('benchmark/reference_runs.json', 'r') as f:
    runs = json.load(f)

with open('benchmark/martin_moyce.json', 'r') as f:
    mm = json.load(f)

Z_225 = mm['scales']['a_2p25_in']['mean']['Z']
T_225_mean = mm['scales']['a_2p25_in']['mean']['T']

Z_1125 = mm['scales']['a_1p125_in']['mean']['Z']
T_1125_mean = mm['scales']['a_1p125_in']['mean']['T']

# Build max/min band for 2.25in
T_225_min = [float('inf')] * len(Z_225)
T_225_max = [-float('inf')] * len(Z_225)
band_map = {}
for rec in mm['scales']['a_2p25_in']['records']:
    for z, t in zip(rec['Z'], rec['T']):
        if z not in band_map: band_map[z] = []
        band_map[z].append(t)

for i, z in enumerate(Z_225):
    if z in band_map:
        T_225_min[i] = min(band_map[z])
        T_225_max[i] = max(band_map[z])
    else:
        T_225_min[i] = T_225_mean[i]
        T_225_max[i] = T_225_mean[i]

def get_run(label_query):
    for r in runs:
        if label_query == r['header']['label']:
            return r
    return None

run_labels = [
    "Van Leer FS N=16 (VOF 4x)",
    "Van Leer FS N=16",
    "Van Leer FS N=32 (dt/4)",
    "First-Order FS N=16",
    "Van Leer No-Slip N=16"
]

def interp(z, Z_arr, T_arr):
    for i in range(1, len(Z_arr)):
        if Z_arr[i-1] <= z <= Z_arr[i]:
            if Z_arr[i] == Z_arr[i-1]: return T_arr[i]
            f = (z - Z_arr[i-1]) / (Z_arr[i] - Z_arr[i-1])
            return T_arr[i-1] + f * (T_arr[i] - T_arr[i-1])
    return None

def analyze(run, Z_exp, T_exp, name, T_min=None, T_max=None):
    print(f"\n--- {run['header']['label']} vs {name} ---")
    print(f"  {'Z':>6} {'T_exp':>6} {'T_sim':>8} {'T_align':>8} {'dT/T%':>7} {'InBand':>6}")
    
    Z_sim = run['Z']
    T_sim = run['T_raw']
    
    anchorT = interp(1.44, Z_sim, T_sim)
    
    rmsRaw7 = 0; maxRaw7 = 0; c7 = 0
    rmsAlign7 = 0; maxAlign7 = 0
    rmsRaw14 = 0; maxRaw14 = 0; c14 = 0
    rmsAlign14 = 0; maxAlign14 = 0
    
    for i, z in enumerate(Z_exp):
        if z > Z_sim[-1]:
            break
        ts = interp(z, Z_sim, T_sim)
        if ts is None: continue
        
        ta = ts - anchorT + 1.19
        te = T_exp[i]
        
        errR = abs(ts - te)/te
        errA = abs(ta - te)/te
        
        inband = "---"
        if T_min is not None and T_max is not None:
            tmin = T_min[i]
            tmax = T_max[i]
            if tmin <= ta <= tmax: inband = "YES"
            else: inband = "NO"
            
        print(f"  {z:6.2f} {te:6.2f} {ts:8.3f} {ta:8.3f} {errA*100:6.1f}% {inband:>6}")
        
        if 1.44 <= z <= 7.00:
            rmsRaw7 += errR*errR
            rmsAlign7 += errA*errA
            maxRaw7 = max(maxRaw7, errR)
            maxAlign7 = max(maxAlign7, errA)
            c7 += 1
            
        if 1.44 <= z <= 14.00:
            rmsRaw14 += errR*errR
            rmsAlign14 += errA*errA
            maxRaw14 = max(maxRaw14, errR)
            maxAlign14 = max(maxAlign14, errA)
            c14 += 1
            
    if c7 > 0:
        print(f"  -> [1.44, 7]  RMS_raw={math.sqrt(rmsRaw7/c7)*100:.1f}% Max_raw={maxRaw7*100:.1f}% | RMS_align={math.sqrt(rmsAlign7/c7)*100:.1f}% Max_align={maxAlign7*100:.1f}%")
    if c14 > 0:
        print(f"  -> [1.44, 14] RMS_raw={math.sqrt(rmsRaw14/c14)*100:.1f}% Max_raw={maxRaw14*100:.1f}% | RMS_align={math.sqrt(rmsAlign14/c14)*100:.1f}% Max_align={maxAlign14*100:.1f}%")

import sys
orig_stdout = sys.stdout
with open('benchmark_output.txt', 'w') as f:
    sys.stdout = f
    for i, l in enumerate(run_labels):
        r = get_run(l)
        if r:
            analyze(r, Z_225, T_225_mean, "a=2.25in", T_225_min, T_225_max)
            analyze(r, Z_1125, T_1125_mean, "a=1.125in")

sys.stdout = orig_stdout

plt.figure(figsize=(10,6))
plt.fill_between(Z_225, T_225_min, T_225_max, color='gray', alpha=0.3, label='Exp Band (2.25in)')
plt.plot(Z_225, T_225_mean, 'ko-', label='Exp Mean (2.25in)')
plt.plot(Z_1125, T_1125_mean, 'ks--', label='Exp Mean (1.125in)')

colors = ['blue', 'red', 'green', 'purple', 'orange']
for i, l in enumerate(run_labels):
    r = get_run(l)
    if r:
        anchorT = interp(1.44, r['Z'], r['T_raw'])
        T_align = [t - anchorT + 1.19 for t in r['T_raw']]
        plt.plot(r['Z'], T_align, label=r['header']['label'], color=colors[i%len(colors)], linewidth=2)

plt.xlabel('Z = x / a')
plt.ylabel('T = t * sqrt(2g / a)')
plt.legend()
plt.title('Dam Break Front Propagation vs Martin & Moyce (1952)')
plt.grid(True)
plt.xlim(0, 15)
plt.ylim(0, 12)
plt.savefig('benchmark_overlay.png')
