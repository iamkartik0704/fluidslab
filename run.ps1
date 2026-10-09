param (
    [string]$Task = "help"
)

# Ensure we are in the script's directory
Set-Location $PSScriptRoot

switch ($Task) {
    "finalbench" {
        Write-Host "Running Dam Break Reference JSON Generator..." -ForegroundColor Cyan
        go run cmd/finalbench/main.go
    }
    "finalpass" {
        Write-Host "Running Grid & Time-Step Convergence Series / Attribution Matrix..." -ForegroundColor Cyan
        go run cmd/finalpass/main.go
    }
    "cavityprint" {
        Write-Host "Running Cavity Benchmark Diagnostics & Profiles..." -ForegroundColor Cyan
        go run cmd/cavityprint/main.go
    }
    "fs_check" {
        Write-Host "Running Free-Slip vs No-Slip Sanity Checks..." -ForegroundColor Cyan
        go run cmd/fs_check/main.go
    }
    "driftdiag" {
        Write-Host "Running Volume Drift / Conservation Verification..." -ForegroundColor Cyan
        go run cmd/driftdiag/main.go
    }
    "serve" {
        Write-Host "Starting UI Server and opening frontend in browser..." -ForegroundColor Cyan
        go run cmd/dambreak/main.go -mode serve
    }
    "test" {
        Write-Host "Running all tests..." -ForegroundColor Cyan
        go test ./internal/solver -run TestPoisson -v
        go test ./internal/benchmark -v
    }
    "all" {
        Write-Host "Running all simulations and tests sequentially..." -ForegroundColor Cyan
        go run cmd/finalbench/main.go
        go run cmd/finalpass/main.go
        go run cmd/cavityprint/main.go
        go run cmd/fs_check/main.go
        go run cmd/driftdiag/main.go
        go test ./internal/solver -run TestPoisson -v
        go test ./internal/benchmark -v
    }
    "help" {
        Write-Host "FluidsLab Runner Script" -ForegroundColor Green
        Write-Host "Usage: .\run.ps1 [task]"
        Write-Host ""
        Write-Host "Available tasks:" -ForegroundColor Yellow
        Write-Host "  finalbench  - Run Dam Break Reference JSON Generator"
        Write-Host "  finalpass   - Run Grid & Time-Step Convergence Series / Attribution Matrix"
        Write-Host "  cavityprint - Run Cavity Benchmark Diagnostics & Profiles"
        Write-Host "  fs_check    - Run Free-Slip vs No-Slip Sanity Checks"
        Write-Host "  driftdiag   - Run Volume Drift / Conservation Verification"
        Write-Host "  serve       - Start the UI Server and load the frontend in browser"
        Write-Host "  test        - Run all logic and validation tests"
        Write-Host "  all         - Run all of the above simulations and tests"
        Write-Host "  help        - Show this help menu (default)"
    }
    default {
        Write-Host "Unknown task: $Task" -ForegroundColor Red
        Write-Host "Run '.\run.ps1 help' for a list of valid tasks."
    }
}
