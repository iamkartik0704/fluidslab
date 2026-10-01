import matplotlib.pyplot as plt
import json
import numpy as np
import subprocess
import os

# Load benchmark data
with open('benchmark/martin_moyce.json', 'r') as f:
    data = json.load(f)
    
anchorZ = data['time_normalisation']['anchor_Z']
anchorT = data['time_normalisation']['anchor_T']

Z_exp = data['scales']['a_2p25_in']['mean']['Z']
T_exp = data['scales']['a_2p25_in']['mean']['T']

def plot_overlay(raw=True):
    plt.figure(figsize=(10, 6))
    plt.plot(T_exp, Z_exp, 'ko-', label='Martin & Moyce (Exp)', linewidth=2)
    
    # We will read from final_output.txt the printed tables.
    # Actually, it's much easier to have a Go program dump Z/T directly to CSV.
    pass

if __name__ == "__main__":
    print("Script ready.")
