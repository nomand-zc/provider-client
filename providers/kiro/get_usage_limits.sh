-- request
curl -X GET "https://q.us-east-1.amazonaws.com/getUsageLimits?isEmailRequired=true&origin=AI_EDITOR&resourceType=AGENTIC_REQUEST" \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN_HERE" \
  -H "x-amz-user-agent: aws-sdk-js/1.0.0 KiroIDE-0.8.140-machine123456" \
  -H "user-agent: aws-sdk-js/1.0.0 ua/2.1 os/Linux lang/js md/nodejs#18.17.0 api/codewhispererruntime#1.0.0 m/E KiroIDE-0.8.140-machine123456" \
  -H "amz-sdk-invocation-id: 550e8400-e29b-41d4-a716-446655440000" \
  -H "amz-sdk-request: attempt=1; max=1" \
  -H "Connection: close"

-- response
{
  "usageLimits": {
    "dailyLimit": 1000,
    "usedToday": 250,
    "remainingToday": 750,
    "monthlyLimit": 30000,
    "usedThisMonth": 12500,
    "remainingThisMonth": 17500,
    "resetTime": "2026-03-06T00:00:00Z",
    "isEmailRequired": true,
    "resourceType": "AGENTIC_REQUEST",
    "accountStatus": "ACTIVE",
    "tier": "STANDARD"
  },
  "accountInfo": {
    "email": "user@example.com",
    "accountId": "123456789012",
    "region": "us-east-1",
    "createdAt": "2024-01-15T10:30:00Z"
  }
}

-- 401 Unauthorized
{
  "error": "Unauthorized",
  "message": "Invalid or expired access token"
}

-- 403 Forbidden
{
  "error": "Forbidden", 
  "message": "Account temporarily suspended"
} 

-- 429 Too Many Requests
{
  "error": "RateLimitExceeded",
  "message": "Too many requests, please try again later",
  "retryAfter": 60
}
