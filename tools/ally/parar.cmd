@echo off
rem YourFlix en el ROG Ally: para la tarea y el servidor (la tarea sola dejaría vivo yourflix.exe).
rem Para volver a arrancarlo sin reiniciar: schtasks /run /tn YourFlix
schtasks /end /tn YourFlix >nul 2>&1
taskkill /f /im yourflix.exe >nul 2>&1
echo YourFlix parado.
