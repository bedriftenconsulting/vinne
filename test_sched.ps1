$lr = Invoke-RestMethod -Uri "http://localhost:4000/api/v1/admin/auth/login" -Method POST -ContentType "application/json" -Body '{"email":"superadmin@randco.com","password":"Admin@123!"}'
$t = $lr.data.access_token

$weekStart = (Get-Date).ToString("yyyy-MM-dd")
$body = '{"week_start":"' + $weekStart + '"}'
$res = Invoke-RestMethod -Uri "http://localhost:4000/api/v1/admin/scheduling/weekly/generate" -Method POST -ContentType "application/json" -Headers @{Authorization="Bearer $t"} -Body $body
Write-Host "Total schedules created: $($res.data.schedules_created)"
$res.data.schedules | ForEach-Object {
    $drawTs = $_.scheduled_draw
    $drawSec = if ($drawTs -is [PSCustomObject]) { $drawTs.seconds } else { 0 }
    $drawDate = [DateTimeOffset]::FromUnixTimeSeconds($drawSec).ToString("ddd yyyy-MM-dd HH:mm")
    Write-Host "  $($_.game_name) | $drawDate | $($_.status)"
}
