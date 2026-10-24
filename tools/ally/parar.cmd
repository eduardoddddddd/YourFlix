@echo off
rem YourFlix en el ROG Ally: para la tarea y le pide al servidor que se cierre (sin ser administrador).
rem El servidor corre en otra sesión de Windows y taskkill no puede con él; por eso se le pide por la admin.
rem Para volver a arrancarlo: schtasks /run /tn YourFlix
schtasks /end /tn YourFlix >nul 2>&1
call "%~dp0reiniciar.cmd" >nul 2>&1
echo YourFlix parado.
