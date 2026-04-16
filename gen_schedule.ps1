$lr2 = Invoke-RestMethod -Uri "http://localhost:4000/api/v1/admin/auth/login" -Method POST -ContentType "application/json" -Body '{"email":"superadmin@randco.com","password":"Admin@123!"}'
$t2 = $lr2.data.access_token

$weekStart = (Get-Date).ToString("yyyy-MM-dd")
$schedBody = '{"week_start":"' + $weekStart + '"}'
$res = Invoke-RestMethod -Uri "http://localhost:4000/api/v1/admin/scheduling/weekly/generate" -Method POST -ContentType "application/json" -Headers @{Authorization="Bearer $t2"} -Body $schedBody
Write-Host "Schedules created: $($res.data.schedules_created)"
$res.data.schedules | ForEach-Object { Write-Host "  $($_.game_name) | $($_.id) | $($_.status)" }
