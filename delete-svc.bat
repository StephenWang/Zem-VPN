Stop-Service -Name ZemCoreSvc -Force
sc.exe delete ZemCoreSvc
Get-Process Zem -ErrorAction SilentlyContinue | Stop-Process -Force