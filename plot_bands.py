import json
import pandas as pd
import numpy as np
import matplotlib.pyplot as plt
import os

with open('benchmark/martin_moyce.json', 'r') as f:
    data = json.load(f)

z_anchor = data['time_normalisation']['anchor_Z']
t_anchor = data['time_normalisation']['anchor_T']

def extract_band(a_key):
    records = data['scales'][a_key]['records']
    all_z = data['scales'][a_key]['mean']['Z']
    min_t = np.full(len(all_z), np.inf)
    max_t = np.full(len(all_z), -np.inf)
    
    for r in records:
        z = np.array(r['Z'])
        t = np.array(r['T'])
        for i, zz in enumerate(all_z):
            if zz >= z[0] and zz <= z[-1]:
                ti = np.interp(zz, z, t)
                if ti < min_t[i]: min_t[i] = ti
                if ti > max_t[i]: max_t[i] = ti
    return all_z, min_t, max_t

plt.figure(figsize=(10, 8))

# Plot bands
z2, min_t2, max_t2 = extract_band('a_2p25_in')
plt.fill_between(z2, min_t2, max_t2, color='gray', alpha=0.3, label='Martin & Moyce (a=2.25" band)')
plt.plot(z2, data['scales']['a_2p25_in']['mean']['T'], 'k-', label='M&M (a=2.25" mean)')

z1, min_t1, max_t1 = extract_band('a_1p125_in')
plt.fill_between(z1, min_t1, max_t1, color='lightblue', alpha=0.3, label='Martin & Moyce (a=1.125" band)')
plt.plot(z1, data['scales']['a_1p125_in']['mean']['T'], 'b--', label='M&M (a=1.125" mean)')

# Plot simulation runs
runs = [
    ('N16_FStrue_VLtrue_dt4', 'VL FS N=16 (dt/4)', 'r-'),
    ('N32_FStrue_VLtrue_dt4', 'VL FS N=32 (dt/4)', 'm-'),
    ('N16_FStrue_VLfalse', 'FO FS N=16 (Ref)', 'g-'),
    ('N16_FSfalse_VLtrue', 'VL NS N=16', 'c-'),
]

for csv_name, label, style in runs:
    path = f'out/finalpass/{csv_name}.csv'
    if os.path.exists(path):
        df = pd.read_csv(path, comment='#')
        z_sim = df['X_star_05'].values
        t_sim = df['t_star'].values
        
        # align
        t_at_anchor = np.interp(z_anchor, z_sim, t_sim)
        t_aligned = t_sim - t_at_anchor + t_anchor
        
        plt.plot(z_sim, t_aligned, style, label=label, linewidth=2)

plt.xlabel('Z (Front Position / a)')
plt.ylabel('T (Normalised Time)')
plt.title('Dam Break Front Propagation vs Martin & Moyce')
plt.grid(True)
plt.legend()
plt.xlim(1.0, 14.0)
plt.ylim(0, 11)
plt.savefig('bench_overlay.png', dpi=300, bbox_inches='tight')
print("Saved bench_overlay.png")
