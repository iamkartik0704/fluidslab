import matplotlib.pyplot as plt
import json
import re

# Load benchmark JSON
with open('benchmark/martin_moyce.json', 'r') as f:
    data = json.load(f)
    
mean_Z = data['scales']['a_2p25_in']['mean']['Z']
mean_T = data['scales']['a_2p25_in']['mean']['T']

plt.figure(figsize=(10, 6))

# Plot experiment data
plt.scatter(mean_Z, mean_T, color='black', label='Experiment (Mean a=2.25")', zorder=5)

with open('attr_output.txt', 'r') as f:
    content = f.read()

colors = {'Baseline (0.5 crossing)': 'blue',
          'Van Leer (0.5 crossing)': 'green',
          '32 cells/L0 (0.5 crossing)': 'red',
          'Density ratio 100': 'orange',
          'Half DT': 'purple'}

current_name = None
z_vals = []
t_sim_vals = []
t_exp_vals = []

for line in content.split('\n'):
    m = re.match(r'--- (.*?) ---', line)
    if m:
        if current_name and current_name in colors:
            plt.plot(z_vals, t_sim_vals, label=current_name, color=colors[current_name])
        current_name = m.group(1)
        z_vals = []
        t_sim_vals = []
        t_exp_vals = []
        continue
    
    m2 = re.match(r'\s*Z=([\d\.]+): T_exp=([\d\.]+) T_sim=([\d\.]+)', line)
    if m2:
        z_vals.append(float(m2.group(1)))
        t_exp_vals.append(float(m2.group(2)))
        t_sim_vals.append(float(m2.group(3)))

if current_name and current_name in colors:
    plt.plot(z_vals, t_sim_vals, label=current_name, color=colors[current_name])

plt.xlabel('Z = z/a')
plt.ylabel('T = t * sqrt(2g/a)')
plt.title('Dam Break Front Position: Simulation vs Martin & Moyce (1952)')
plt.legend(bbox_to_anchor=(1.05, 1), loc='upper left')
plt.grid(True)
plt.tight_layout()
plt.savefig(r'C:\Users\iamka\.gemini\antigravity-ide\brain\2509ce5c-3666-4963-8881-f8d219857e17\bench_overlay.png')
print("Plot saved.")

# Also write CSV
with open(r'C:\Users\iamka\.gemini\antigravity-ide\brain\2509ce5c-3666-4963-8881-f8d219857e17\bench_data.csv', 'w') as f:
    f.write("Name,Z,T_exp,T_sim\n")
    current_name = None
    for line in content.split('\n'):
        m = re.match(r'--- (.*?) ---', line)
        if m:
            current_name = m.group(1)
            continue
        m2 = re.match(r'\s*Z=([\d\.]+): T_exp=([\d\.]+) T_sim=([\d\.]+)', line)
        if m2 and current_name:
            f.write(f"{current_name},{m2.group(1)},{m2.group(2)},{m2.group(3)}\n")
