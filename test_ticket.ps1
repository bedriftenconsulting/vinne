$pwd = 'P5r2n4x3100$$'
$body = "{`"phone_number`":`"+233256826832`",`"password`":`"$pwd`",`"device_id`":`"web_test`",`"channel`":`"WEBSITE`",`"device_info`":{`"device_type`":`"web`",`"os`":`"web`",`"os_version`":`"web`",`"app_version`":`"1.0.0`",`"user_agent`":`"test`"}}"
$lr = Invoke-RestMethod -Uri "http://localhost:4000/api/v1/players/login" -Method POST -ContentType "application/json" -Body $body
$t = $lr.access_token
$uid = $lr.profile.id

# Use PS5 schedule ID
$scheduleId = "cb09018b-e52b-46f4-9ae1-5a7fbd197aa2"

$ticketBody = '{"game_code":"PS5DRAW","game_schedule_id":"' + $scheduleId + '","draw_number":1,"selected_numbers":[],"bet_lines":[{"line_number":1,"bet_type":"RAFFLE","selected_numbers":[],"total_amount":1000}],"customer_phone":"+233256826832","customer_name":"Suraj Mohammed","payment_method":"mobile_money","payment_ref":"manual-test-123"}'

try {
    $res = Invoke-RestMethod -Uri "http://localhost:4000/api/v1/players/$uid/tickets" -Method POST -ContentType "application/json" -Headers @{Authorization="Bearer $t"} -Body $ticketBody
    Write-Host "SUCCESS"
    $res | ConvertTo-Json -Depth 5
} catch {
    Write-Host "FAILED: $($_.ErrorDetails.Message)"
}
