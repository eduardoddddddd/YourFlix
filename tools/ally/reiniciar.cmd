@echo off
rem YourFlix en el ROG Ally: le pide al servidor que se cierre; iniciar.cmd lo relanza en 3 s (con el
rem yourflix.exe que haya en ese momento: así se actualiza). Corta lo que se esté viendo en la TV.
rem Solo funciona desde el propio Ally (la admin deja entrar sin contraseña desde localhost).
powershell -NoProfile -Command "$s = Invoke-RestMethod http://localhost:8787/admin/api/session -SessionVariable w; Invoke-RestMethod -Method Post http://localhost:8787/admin/api/restart -WebSession $w -ContentType 'application/json' -Headers @{ 'X-CSRF-Token' = $s.csrf } -Body '{}' | Out-Null; 'Reinicio pedido.'"
