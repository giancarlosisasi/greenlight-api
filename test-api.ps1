# 1..8 | ForEach-Object -Parallel {
#     curl -X PATCH -d '{"runtime": "97 mins"}' "localhost:4000/v1/movies/5cc91017-402b-4d09-b03a-03b7dbb8b2b4"
# } -ThrottleLimit 8

# # Get movie
# 1..20 | ForEach-Object -Parallel {
#     curl "localhost:4000/v1/healthcheck"
# }

# Parallel HTTP Requests Script
# Usage: .\parallel-requests.ps1 -Url "http://localhost:4000/v1/healthcheck" -Count 20
