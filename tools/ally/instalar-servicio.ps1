﻿# YourFlix en el ROG Ally: arranque automático del servidor. Ejecutar UNA vez EN EL ALLY como administrador:
# doble clic en C:\YourFlix\servidor\instalar.cmd (pide permiso de Windows), o en PowerShell como administrador:
#   powershell -ExecutionPolicy Bypass -File C:\YourFlix\servidor\instalar-servicio.ps1
# Hace tres cambios en Windows (y nada más):
#   1. Tarea programada «YourFlix»: al encender, como SYSTEM, sin iniciar sesión; iniciar.cmd relanza el servidor si se cae.
#   2. Regla de firewall «YourFlix (TCP 8787)»: entrada solo por el puerto 8787 y solo para C:\YourFlix\servidor\yourflix.exe.
#      La Wi-Fi del Ally está como red Pública, así que vale para Privada y Pública.
#   3. Energía con cargador: no suspender ni hibernar nunca (la pantalla sí puede apagarse).
# No arranca el servidor: lo hará el próximo encendido o «schtasks /run /tn YourFlix».
# Deja lo que pasó en C:\YourFlix\servidor\instalar.log (lo lee Claude).
#Requires -RunAsAdministrator
$ErrorActionPreference = 'Stop'
$dir = 'C:\YourFlix\servidor'
$exe = Join-Path $dir 'yourflix.exe'
Start-Transcript -Path (Join-Path $dir 'instalar.log') -Force | Out-Null
try {
  if (-not (Test-Path $exe)) { throw "No existe $exe" }

  $action = New-ScheduledTaskAction -Execute 'cmd.exe' -Argument "/c `"$dir\iniciar.cmd`"" -WorkingDirectory $dir
  $trigger = New-ScheduledTaskTrigger -AtStartup
  $principal = New-ScheduledTaskPrincipal -UserId 'SYSTEM' -LogonType ServiceAccount -RunLevel Highest
  $settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable -ExecutionTimeLimit ([TimeSpan]::Zero) -MultipleInstances IgnoreNew
  Register-ScheduledTask -TaskName 'YourFlix' -Action $action -Trigger $trigger -Principal $principal -Settings $settings -Description 'Servidor YourFlix (películas para la TV y la web), puerto 8787' -Force | Out-Null
  Write-Host '1/3 Tarea programada YourFlix creada (al encender, como SYSTEM).'

  Get-NetFirewallRule -DisplayName 'YourFlix (TCP 8787)' -ErrorAction SilentlyContinue | Remove-NetFirewallRule
  New-NetFirewallRule -DisplayName 'YourFlix (TCP 8787)' -Direction Inbound -Action Allow -Protocol TCP -LocalPort 8787 -Program $exe -Profile Private,Public | Out-Null
  Write-Host '2/3 Regla de firewall YourFlix (TCP 8787) creada.'

  powercfg /change standby-timeout-ac 0
  powercfg /change hibernate-timeout-ac 0
  Write-Host '3/3 Con cargador: sin suspensión ni hibernación.'
  Write-Host 'LISTO. Avisa a Claude para pasar los datos y arrancar el servidor nuevo.'
} catch {
  Write-Host "ERROR: $($_.Exception.Message)"
} finally {
  Stop-Transcript | Out-Null
  Read-Host 'Pulsa Intro para cerrar'
}
