import pandas as pd
import matplotlib.pyplot as plt
import json

# Load benchmark JSON
with open('benchmark/martin_moyce.json', 'r') as f:
    data = json.load(f)
    
mean_Z = data['scales']['a_2p25_in']['mean']['Z']
mean_T = data['scales']['a_2p25_in']['mean']['T']

# Load simulation data
df = pd.read_csv('bench_data.csv')

plt.figure(figsize=(10, 6))

# Plot experiment data
plt.scatter(mean_Z, mean_T, color='black', label='Experiment (Mean a=2.25")', zorder=5)

colors = {'N16_FSfalse_VLfalse': 'blue', 'N16_FSfalse_VLtrue': 'lightblue',
          'N16_FStrue_VLfalse': 'green', 'N16_FStrue_VLtrue': 'lightgreen',
          'N32_FSfalse_VLtrue': 'red', 'N32_FStrue_VLtrue': 'orange'}

# Plot unaligned
for name in df['Name'].unique():
    subset = df[(df['Name'] == name) & (df['Aligned'] == False)]
    c = colors.get(name, 'gray')
    plt.plot(subset['Z'], subset['T_sim'], label=f'{name} (Raw)', linestyle='--', color=c)
    
# Plot aligned
for name in df['Name'].unique():
    subset = df[(df['Name'] == name) & (df['Aligned'] == True)]
    c = colors.get(name, 'gray')
    plt.plot(subset['Z'], subset['T_sim'], label=f'{name} (Shifted)', linestyle='-', color=c)

plt.xlabel('Z = z/a')
plt.ylabel('T = t * sqrt(2g/a)')
plt.title('Dam Break Front Position: Simulation vs Martin & Moyce (1952)')
plt.legend(bbox_to_anchor=(1.05, 1), loc='upper left')
plt.grid(True)
plt.tight_layout()
plt.savefig('bench_overlay.png')
print("Plot saved as bench_overlay.png")
