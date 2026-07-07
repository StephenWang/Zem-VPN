@echo off
setlocal enabledelayedexpansion
REM build.bat - Windows build script

echo Building Zem...

cd frontend
call npm install
cd ..

go mod tidy

REM stop and remove old/new services if exist
echo Stopping services...
sc.exe stop ZemCoreSvc > nul 2> nul

echo Waiting for services to stop...
for /L %%i in (1,1,10) do (
    timeout /t 1 /nobreak > nul
    sc.exe query ZemService | find /I "STOPPED" > nul 2> nul
    if !errorlevel! == 0 (
        sc.exe query ZemCoreSvc | find /I "STOPPED" > nul 2> nul
        if !errorlevel! == 0 goto :services_stopped
    )
)
:services_stopped

echo Killing Zem.exe processes...
taskkill /F /IM Zem.exe > nul 2> nul
timeout /t 1 /nobreak > nul

echo Deleting services...
sc.exe delete ZemCoreSvc > nul 2> nul

set DIST_DIR=dist
if not exist "%DIST_DIR%" mkdir "%DIST_DIR%"

wails build -ldflags "-s -w -buildid=" -o "%DIST_DIR%\Zem.exe"

if exist "build\bin\dist\Zem.exe" (
    copy /Y "build\bin\dist\Zem.exe" "%DIST_DIR%\Zem.exe" > nul
    echo Copied build\bin\dist\Zem.exe to %DIST_DIR%\Zem.exe
)

if exist "%DIST_DIR%\Zem.exe" (
    echo Build complete: %DIST_DIR%\Zem.exe
) else (
    echo Build failed: %DIST_DIR%\Zem.exe not found
    exit /b 1
)

pause
