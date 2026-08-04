#!/bin/bash
echo "=== Starting Vero Services ==="
echo ""

echo "[1/2] Starting Backend on port 8080..."
./vero.exe -port 8080 > /tmp/vero-backend.log 2>&1 &
BACKEND_PID=$!
echo "Backend PID: $BACKEND_PID"
sleep 2

echo "[2/2] Starting Frontend..."
cd web
npm run dev > /tmp/vero-frontend.log 2>&1 &
FRONTEND_PID=$!
echo "Frontend PID: $FRONTEND_PID"
sleep 2

echo ""
echo "=== Services Started ==="
echo "Backend: http://localhost:8080 (PID: $BACKEND_PID)"
echo "Frontend: http://localhost:5173 or 5174 (PID: $FRONTEND_PID)"
echo ""
echo "Logs:"
echo "  Backend:  tail -f /tmp/vero-backend.log"
echo "  Frontend: tail -f /tmp/vero-frontend.log"
echo ""
echo "Stop services:"
echo "  kill $BACKEND_PID $FRONTEND_PID"
