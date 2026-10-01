import re

with open('cmd/dambreak/main.go', 'r') as f:
    content = f.read()

target = 'fmt.Printf("t*=%4.2f step=%d X*=%5.3f drift=%.2e%%\\n", s.TStar, sim.State.Step, s.FrontXStar, driftPct)'

replacement = '''
			minAlpha := 1.0
			maxAlpha := 0.0
			for _, a := range sim.Fields.Alpha {
				if a < minAlpha { minAlpha = a }
				if a > maxAlpha { maxAlpha = a }
			}
			exceeds := "No"
			if s.FrontXStar > 1 + 2*s.TStar {
				exceeds = "Yes"
			}
			fmt.Printf("| %.2f | %.3f | %.3e | %.3e | %d | %.2e | %.4f | %s |\\n", 
				s.TStar, s.FrontXStar, driftPct, sim.State.MaxDiv, sim.State.PoissonIter, minAlpha, maxAlpha, exceeds)
'''
content = content.replace(target, replacement)

target2 = 'fmt.Printf("Dam break completed in %v\\n", elapsed)'
replacement2 = '''fmt.Printf("Dam break completed in %v\\n", elapsed)
	fmt.Printf("Final Clipped Mass: %.6e\\n", sim.State.ClippedMass)
	fmt.Printf("Final Top Outflow: %.6e\\n", sim.State.TopOutflow)
'''
content = content.replace(target2, replacement2)

with open('cmd/dambreak/main.go', 'w') as f:
    f.write(content)
