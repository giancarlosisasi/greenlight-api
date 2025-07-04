1..8 | ForEach-Object -Parallel {
    curl -X PATCH -d '{"runtime": "97 mins"}' "localhost:4000/v1/movies/5cc91017-402b-4d09-b03a-03b7dbb8b2b4"
} -ThrottleLimit 8