# Idempotency-Key Middleware

A single-node, in-memory idempotency layer for HTTP POST endpoints in Go.
 
 /tmp/final_check/idempotency (main)
$ go test -v ./...
=== RUN   TestIdempotentReplay
--- PASS: TestIdempotentReplay (0.00s)
=== RUN   TestConcurrentRetryBlocksNot409
--- PASS: TestConcurrentRetryBlocksNot409 (0.15s)
=== RUN   TestFingerprintMismatch
--- PASS: TestFingerprintMismatch (0.00s)
=== RUN   TestTransientFailureNotCached
--- PASS: TestTransientFailureNotCached (0.00s)
=== RUN   TestClientErrorCached
--- PASS: TestClientErrorCached (0.00s)
=== RUN   TestNoKeyPassthrough
--- PASS: TestNoKeyPassthrough (0.00s)
=== RUN   TestPanicCleanup
--- PASS: TestPanicCleanup (0.00s)
=== RUN   TestMemoryBounded
--- PASS: TestMemoryBounded (0.00s)
=== RUN   TestRace
--- PASS: TestRace (0.02s)
=== RUN   TestBodyAvailableToHandler
--- PASS: TestBodyAvailableToHandler (0.00s)
PASS
ok      vectorshift_assignment/idempotency      1.047s


$ curl -X POST http://localhost:8080/order -H "Idempotency-Key: abc-123" -H "Content-Type: application/json" -d '{"amount":100}'
  % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                 Dload  Upload   Total   Spent    Left  Speed
100    52  100    38  100    14  21826   8041 --:--:-- --:--:-- --:--:-- 52000{"status":"created","id":"order-123"}


$ curl -X POST http://localhost:8080/order -H "Idempotency-Key: abc-123" -H "Content-Type: application/json" -d '{"amount":100}'
  % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                 Dload  Upload   Total   Spent    Left  Speed
100    52  100    38  100    14  26116   9621 --:--:-- --:--:-- --:--:-- 52000{"status":"created","id":"order-123"}

$ curl -X POST http://localhost:8080/order -H "Idempotency-Key: abc-123" -H "Content-Type: application/json" -d '{"amount":999}'
  % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                 Dload  Upload   Total   Spent    Left  Speed
100    46  100    32  100    14  23021  10071 --:--:-- --:--:-- --:--:-- 46000idempotency key mismatch occurs


$ curl -X POST http://localhost:8080/order -H "Content-Type: application/json" -d '{"amount":1}'
  % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                 Dload  Upload   Total   Spent    Left  Speed
100    50  100    38  100    12  17715   5594 --:--:-- --:--:-- --:--:-- 50000{"status":"created","id":"order-123"}
