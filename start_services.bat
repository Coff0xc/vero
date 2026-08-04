@echo off
echo === Starting Vero Services ===
echo.

echo [1/2] Starting Backend on port 8080...
start "Vero Backend" cmd /k "vero.exe -port 8080"
timeout /t 3 /nobreak >nul

echo [2/2] Starting Frontend on port 5173...
cd web
start "Vero Frontend" cmd /k "npm run dev"
timeout /t 2 /nobreak >nul

echo.
echo === Services Started ===
echo Backend: http://localhost:8080
echo Frontend: http://localhost:5173 (or 5174)
echo.
echo Press any key to close this window...
pause >nul
