import json
import math
import matplotlib.pyplot as plt
import numpy as np

# Load benchmark runs
with open('benchmark/reference_runs.json', 'r') as f:
    runs = json.load(f)

# Experimental data
# a=2.25in
Z_225 = [1.11, 1.22, 1.44, 1.67, 1.89, 2.11, 2.33, 2.56, 2.78, 3.00, 3.22, 3.44, 3.67, 4.00, 4.56, 5.00, 5.56, 6.00, 7.00, 8.00, 9.00, 10.00, 12.00, 14.00, 16.00]
T_225_mean = [0.82, 0.94, 1.25, 1.47, 1.63, 1.76, 1.95, 2.05, 2.18, 2.27, 2.51, 2.56, 2.74, 3.01, 3.39, 3.73, 4.06, 4.39, 5.06, 5.76, 6.43, 7.15, 8.52, 10.02, 11.49]
T_225_min = [0.77, 0.91, 1.15, 1.42, 1.58, 1.74, 1.85, 2.04, 2.16, 2.25, 2.45, 2.55, 2.70, 2.94, 3.34, 3.64, 3.98, 4.35, 4.97, 5.72, 6.40, 7.11, 8.41,  9.88, 11.23]
T_225_max = [0.87, 0.99, 1.34, 1.55, 1.71, 1.81, 2.04, 2.13, 2.22, 2.36, 2.56, 2.65, 2.79, 3.09, 3.43, 3.84, 4.14, 4.45, 5.17, 5.86, 6.54, 7.23, 8.57, 10.15, 11.59]

# a=1.125in
Z_1125 = [1.11, 1.33, 1.56, 1.78, 2.00, 2.22, 2.44, 2.67, 3.11, 3.56, 4.00, 4.44, 4.89, 5.33, 5.78, 6.22, 6.76]
T_1125_mean = [0.93, 1.25, 1.47, 1.72, 1.87, 2.06, 2.25, 2.46, 2.71, 3.05, 3.39, 3.70, 3.97, 4.29, 4.60, 4.96, 5.31]

def get_run(label_query):
    for r in runs:
        if label_query in r['header']['label']:
            return r
    return None

run_labels = [
    "Van Leer FS N=16 Demo",
    "Van Leer FS N=16", # native
    "Van Leer FS N=32 dt/4",
    "First-Order FS N=16",
    "Van Leer No-Slip N=16"
]

def interp(z, Z_arr, T_arr):
    for i in range(1, len(Z_arr)):
        if Z_arr[i-1] <= z <= Z_arr[i]:
            if Z_arr[i] == Z_arr[i-1]:
                return T_arr[i]
            f = (z - Z_arr[i-1]) / (Z_arr[i] - Z_arr[i-1])
            return T_arr[i-1] + f * (T_arr[i] - T_arr[i-1])
    return None

def analyze(run, Z_exp, T_exp, name):
    print(f"\\n--- {run['header']['label']} vs {name} ---")
    print(f"  {'Z':>6} {'T_exp':>6} {'T_sim':>8} {'T_align':>8} {'dT/T%':>7} {'InBand':>6}")
    
    Z_sim = run['Z']
    T_sim = run['T_raw']
    
    # alignment at Z=1.44
    anchorT = interp(1.44, Z_sim, T_sim)
    
    rmsRaw7 = 0; maxRaw7 = 0; c7 = 0
    rmsAlign7 = 0; maxAlign7 = 0
    rmsRaw14 = 0; maxRaw14 = 0; c14 = 0
    rmsAlign14 = 0; maxAlign14 = 0
    
    for i, z in enumerate(Z_exp):
        if z > Z_sim[-1]:
            continue
        
        t_raw = interp(z, Z_sim, T_sim)
        t_align = t_raw - anchorT + 1.25
        
        pctRaw = 100 * (t_raw - T_exp[i]) / T_exp[i]
        pctAlign = 100 * (t_align - T_exp[i]) / T_exp[i]
        
        inBand = "NO"
        if name == "a=2.25in":
            bMin = T_225_min[i]
            bMax = T_225_max[i]
            if bMin <= t_align <= bMax:
                inBand = "YES"
                
        print(f"  {z:6.2f} {T_exp[i]:6.2f} {t_raw:8.3f} {t_align:8.3f} {pctAlign:+6.1f}% {inBand:>6}")
        
        if 1.44 <= z <= 7.0:
            rmsRaw7 += pctRaw*pctRaw
            rmsAlign7 += pctAlign*pctAlign
            c7 += 1
            maxRaw7 = max(maxRaw7, abs(pctRaw))
            maxAlign7 = max(maxAlign7, abs(pctAlign))
            
        if 1.44 <= z <= 14.0:
            rmsRaw14 += pctRaw*pctRaw
            rmsAlign14 += pctAlign*pctAlign
            c14 += 1
            maxRaw14 = max(maxRaw14, abs(pctRaw))
            maxAlign14 = max(maxAlign14, abs(pctAlign))

    if c7 > 0:
        print(f"  -> [1.44, 7]  RMS_raw={math.sqrt(rmsRaw7/c7):.1f}% Max_raw={maxRaw7:.1f}% | RMS_align={math.sqrt(rmsAlign7/c7):.1f}% Max_align={maxAlign7:.1f}%")
    if c14 > 0:
        print(f"  -> [1.44, 14] RMS_raw={math.sqrt(rmsRaw14/c14):.1f}% Max_raw={maxRaw14:.1f}% | RMS_align={math.sqrt(rmsAlign14/c14):.1f}% Max_align={maxAlign14:.1f}%")

with open('benchmark_output.txt', 'w') as f:
    import sys
    sys.stdout = f
    
    for l in run_labels:
        r = get_run(l)
        if r:
            analyze(r, Z_225, T_225_mean, "a=2.25in")
            analyze(r, Z_1125, T_1125_mean, "a=1.125in")

# Make plot
plt.figure(figsize=(10, 6))
plt.fill_between(Z_225, T_225_min, T_225_max, color='gray', alpha=0.3, label='Experimental Band (a=2.25in)')
plt.plot(Z_225, T_225_mean, 'ko-', label='Experimental Mean (a=2.25in)')

colors = ['r', 'b', 'g', 'c', 'm']
for i, l in enumerate(run_labels):
    r = get_run(l)
    if r:
        anchorT = interp(1.44, r['Z'], r['T_raw'])
        T_align = [t - anchorT + 1.25 for t in r['T_raw']]
        plt.plot(r['Z'], T_align, label=r['header']['label'], color=colors[i%len(colors)], linewidth=2)

plt.xlabel('Z = x / a')
plt.ylabel('T = t * sqrt(2g / a)')
plt.title('Dam Break Front Propagation Benchmark')
plt.legend()
plt.grid(True)
plt.savefig('benchmark_overlay.png')
