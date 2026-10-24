@echo off
rem YourFlix en el ROG Ally: arranca el servidor y lo relanza si se cae.
rem Lo ejecuta la tarea programada «YourFlix» al encender (instalar-servicio.ps1).
rem Todo vive junto a este archivo: yourflix.exe, ffmpeg.exe, ffprobe.exe, data\ y secrets\.
cd /d "%~dp0"
:loop
if exist data\yourflix.log for %%F in (data\yourflix.log) do if %%~zF GTR 10485760 move /y data\yourflix.log data\yourflix.old.log >nul
echo %date% %time% arranque >> data\yourflix.log
yourflix.exe --data "%~dp0data" --tmdb-token-file "%~dp0secrets\tmdb.token" serve >> data\yourflix.log 2>&1
echo %date% %time% el servidor se ha parado (codigo %errorlevel%); se relanza en 10 s >> data\yourflix.log
timeout /t 10 /nobreak >nul
goto loop
