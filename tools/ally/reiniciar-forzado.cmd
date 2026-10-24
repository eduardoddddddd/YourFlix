@echo off
rem YourFlix en el ROG Ally: doble clic si el servidor no responde y reiniciar.cmd no sirve.
rem Pide permiso de administrador, mata yourflix.exe y vuelve a lanzar la tarea.
powershell -NoProfile -Command "Start-Process cmd.exe -Verb RunAs -ArgumentList '/c taskkill /f /im yourflix.exe & schtasks /end /tn YourFlix & schtasks /run /tn YourFlix & timeout /t 3'"
