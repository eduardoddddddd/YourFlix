@echo off
rem YourFlix en el ROG Ally: arranca el servidor y lo relanza si se cae.
rem Lo ejecuta la tarea programada «YourFlix» al encender (instalar-servicio.ps1).
rem Todo vive junto a este archivo: yourflix.exe, ffmpeg.exe, ffprobe.exe, data\ y secrets\.
cd /d "%~dp0"
rem Si quedó un yourflix.exe huérfano (p. ej. tras «schtasks /end»), cerrarlo: corre en la misma sesión que esta tarea.
taskkill /f /im yourflix.exe >nul 2>&1
:loop
if exist data\yourflix.log for %%F in (data\yourflix.log) do if %%~zF GTR 10485760 move /y data\yourflix.log data\yourflix.old.log >nul
echo %date% %time% arranque >> data\yourflix.log
yourflix.exe --data "%~dp0data" --tmdb-token-file "%~dp0secrets\tmdb.token" serve >> data\yourflix.log 2>&1
echo %date% %time% el servidor se ha parado (codigo %errorlevel%); se relanza en 3 s >> data\yourflix.log
timeout /t 3 /nobreak >nul
goto loop
