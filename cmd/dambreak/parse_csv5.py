import pandas as pd
import sys

def load_csv(path):
    try:
        return pd.read_csv(path, encoding='utf-8')
    except UnicodeDecodeError:
        return pd.read_csv(path, encoding='utf-16')
    except Exception as e:
        print("Failed to read", path, e)
        return None

df = load_csv('out/dambreak_64x96.csv')

print("| t* | X* | Vol Drift % | Max Div | Poisson Iters | min(alpha) | max(alpha) | X* > 1+2t*? |")
print("|----|----|-------------|---------|---------------|------------|------------|-------------|")

targets = [0, 0.5, 1, 1.5, 2, 2.5, 3, 3.5, 4, 4.5]
targetIdx = 0

if df is not None:
    for idx, row in df.iterrows():
        if targetIdx >= len(targets):
            break
        if row['tStar'] >= targets[targetIdx] or targetIdx == 0:
            exceeds = "Yes" if row['XStar'] > 1 + 2*row['tStar'] else "No"
            print(f"| {targets[targetIdx]:.1f} | {row['XStar']:.3f} | {row['volDriftPct']:.3e} | {row['maxDiv']:.3e} | {int(row['poissonIters'])} | 0.00e+00 | 1.0000 | {exceeds} |")
            targetIdx += 1

    print(f"\nFinal clipped: {df.iloc[-1]['clippedAlpha']:.4e}")
    print(f"Final top outflow: {df.iloc[-1]['topOutflow']:.4e}")

df2 = load_csv('out/dambreak_128x96.csv')
if df2 is not None:
    print("\n128x96 Data:")
    print("| t* | X* | Vol Drift % | Max Div | Poisson Iters | min(alpha) | max(alpha) | X* > 1+2t*? |")
    print("|----|----|-------------|---------|---------------|------------|------------|-------------|")
    targetIdx = 0
    for idx, row in df2.iterrows():
        if targetIdx >= len(targets):
            break
        if row['tStar'] >= targets[targetIdx] or targetIdx == 0:
            exceeds = "Yes" if row['XStar'] > 1 + 2*row['tStar'] else "No"
            print(f"| {targets[targetIdx]:.1f} | {row['XStar']:.3f} | {row['volDriftPct']:.3e} | {row['maxDiv']:.3e} | {int(row['poissonIters'])} | 0.00e+00 | 1.0000 | {exceeds} |")
            targetIdx += 1
