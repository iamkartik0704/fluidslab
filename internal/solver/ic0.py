import re

with open('poisson.go', 'r') as f:
    content = f.read()

# I will replace the Jacobi preconditioner with IC(0).
# We can use scratch[7] for invE if we update scratch length, but wait!
# Let's see how many arrays we have in scratch.
