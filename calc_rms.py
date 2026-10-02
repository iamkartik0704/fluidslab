import json
import pandas as pd
import numpy as np

with open('benchmark/martin_moyce.json', 'r') as f:
    data = json.load(f)

z_anchor = data['time_normalisation']['anchor_Z']
t_anchor = data['time_normalisation']['anchor_T']

runs = [
    ('N16_FStrue_VLtrue_dt4', 'VL FS N=16 dt4'),
    ('N32_FStrue_VLtrue_dt4', 'VL FS N=32 dt4'),
    ('N16_FStrue_VLfalse', 'FO FS N=16'),
    ('N16_FSfalse_VLtrue', 'VL NS N=16')
]

for csv_name, label in runs:
    df = pd.read_csv(f'out/finalpass/{csv_name}.csv', comment='#')
    z_sim = df['X_star_05'].values
    t_sim = df['t_star'].values
    
    t_at_anchor = np.interp(z_anchor, z_sim, t_sim)
    t_aligned = t_sim - t_at_anchor + t_anchor
    
    print(f"\n--- {label} ---")
    
    for a_key, a_label in [('a_2p25_in', 'a=2.25'), ('a_1p125_in', 'a=1.125')]:
        z_exp = data['scales'][a_key]['mean']['Z']
        t_exp = data['scales'][a_key]['mean']['T']
        
        rms_raw7 = rms_aligned7 = 0
        rms_raw14 = rms_aligned14 = 0
        c7 = c14 = 0
        max_raw7 = max_aligned7 = 0
        max_raw14 = max_aligned14 = 0
        
        print(f"  [{a_label}]")
        for i, z in enumerate(z_exp):
            if z < 1.44 or z > z_sim[-1]: continue
            t_s = np.interp(z, z_sim, t_sim)
            t_a = np.interp(z, z_sim, t_aligned)
            dt_raw = (t_s - t_exp[i]) / t_exp[i] * 100
            dt_aligned = (t_a - t_exp[i]) / t_exp[i] * 100
            
            # calculate band membership? I'll just print RMS for now
            if z <= 7.0:
                rms_raw7 += dt_raw**2; rms_aligned7 += dt_aligned**2; c7 += 1
                max_raw7 = max(max_raw7, abs(dt_raw)); max_aligned7 = max(max_aligned7, abs(dt_aligned))
            if z <= 14.0:
                rms_raw14 += dt_raw**2; rms_aligned14 += dt_aligned**2; c14 += 1
                max_raw14 = max(max_raw14, abs(dt_raw)); max_aligned14 = max(max_aligned14, abs(dt_aligned))
                
        if c7>0:
            print(f"    -> [1.44, 7]  RMS_raw={np.sqrt(rms_raw7/c7):.1f}% Max_raw={max_raw7:.1f}% | RMS_aligned={np.sqrt(rms_aligned7/c7):.1f}% Max_aligned={max_aligned7:.1f}%")
        if c14>0:
            print(f"    -> [1.44, 14] RMS_raw={np.sqrt(rms_raw14/c14):.1f}% Max_raw={max_raw14:.1f}% | RMS_aligned={np.sqrt(rms_aligned14/c14):.1f}% Max_aligned={max_aligned14:.1f}%")

