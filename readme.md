* Supports creating custom rules in any mode.
* Implemented using a memory token bucket.
* easy use. u can add and any web server(eg:gin) as middleware

```golang
	Init_rate_limit(keys_check_minute, keys_check num) //init rate limiter and start keys check

	AppAddRule(pattern, limit, duration) // define you pattern at duration seconds can access limit time

	AppTokenAccess(key, pattern) //check your key token access with pattern
```
