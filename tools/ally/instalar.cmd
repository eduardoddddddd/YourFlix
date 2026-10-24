@echo off
rem YourFlix en el ROG Ally: doble clic aqui. Windows pide permiso de administrador y ejecuta instalar-servicio.ps1.
powershell -NoProfile -Command "Start-Process powershell -Verb RunAs -ArgumentList @('-NoProfile','-ExecutionPolicy','Bypass','-File','%~dp0instalar-servicio.ps1')"
