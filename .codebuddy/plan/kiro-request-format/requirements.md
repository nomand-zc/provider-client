# Kiro 渠道请求参数格式完整规范文档

## 引言

本文档详细定义了 Kiro（AWS CodeWhisperer）渠道的请求参数完整格式规范。Kiro 渠道采用 AWS CodeWhisperer API 格式，支持单轮对话、多轮对话、工具调用等多种场景。本文档提供详细的请求和响应格式示例，涵盖各种使用场景。

## 基础请求格式

### 通用请求结构

```json
{
  "modelId": "string",           // 模型标识，如 "amazon.codewhisperer"
  "profileArn": "string",        // 用户配置ARN，用于social模式认证
  "conversationState": {         // 对话状态，包含历史消息和当前请求
    "chatTriggerType": "string", // 聊天触发类型
    "conversationId": "string",  // 对话ID，用于多轮对话
    "currentMessage": {          // 当前用户消息
      "content": "string",       // 消息内容
      "userInputMessageContext": { // 用户输入上下文
        "tools": [               // 工具定义列表
          {
            "toolSpecification": {
              "name": "string",
              "description": "string",
              "inputSchema": {
                "type": "object",
                "properties": {},
                "required": []
              }
            }
          }
        ],
        "toolResults": [         // 工具执行结果
          {
            "toolCallId": "string",
            "content": [
              {
                "json": {}
              }
            ]
          }
        ]
      }
    },
    "history": [                // 对话历史
      {
        "userInputMessage": {   // 用户输入消息
          "content": "string",
          "userInputMessageContext": {}
        }
      },
      {
        "assistantResponseMessage": { // AI助手响应消息
          "content": "string",
          "toolCalls": [
            {
              "toolCallId": "string",
              "name": "string",
              "input": {}
            }
          ]
        }
      }
    ]
  }
}
```

### 通用响应结构

```json
{
  "conversationId": "string",    // 对话ID
  "systemMessage": {             // 系统消息（可选）
    "content": "string"
  },
  "messages": [                  // 响应消息列表
    {
      "assistantResponseMessage": { // AI助手响应
        "content": "string",
        "toolCalls": [
          {
            "toolCallId": "string",
            "name": "string",
            "input": {}
          }
        ]
      }
    }
  ],
  "usage": {                     // Token使用统计
    "inputTokens": 0,
    "outputTokens": 0,
    "totalTokens": 0
  },
  "error": {                     // 错误信息（可选）
    "code": "string",
    "message": "string"
  }
}
```

## 场景 1：不带工具调用的单轮对话

### 请求示例

```json
{
  "modelId": "amazon.codewhisperer",
  "profileArn": "arn:aws:codewhisperer:us-east-1:123456789012:profile/user-profile",
  "conversationState": {
    "chatTriggerType": "TEXT",
    "conversationId": "conv-001",
    "currentMessage": {
      "content": "请帮我写一个Python函数来计算斐波那契数列"
    }
  }
}
```

### 响应示例

```json
{
  "conversationId": "conv-001",
  "messages": [
    {
      "assistantResponseMessage": {
        "content": "以下是一个计算斐波那契数列的Python函数：\n\n```python\ndef fibonacci(n):\n    \"\"\"\n    计算斐波那契数列的第n项\n    \"\"\"\n    if n <= 0:\n        return 0\n    elif n == 1:\n        return 1\n    else:\n        a, b = 0, 1\n        for _ in range(2, n + 1):\n            a, b = b, a + b\n        return b\n\n# 示例用法\nprint(fibonacci(10))  # 输出：55\n```"
      }
    }
  ],
  "usage": {
    "inputTokens": 15,
    "outputTokens": 120,
    "totalTokens": 135
  }
}
```

## 场景 2：不带工具调用的多轮对话

### 请求示例（第二轮对话）

```json
{
  "modelId": "amazon.codewhisperer",
  "profileArn": "arn:aws:codewhisperer:us-east-1:123456789012:profile/user-profile",
  "conversationState": {
    "chatTriggerType": "TEXT",
    "conversationId": "conv-002",
    "currentMessage": {
      "content": "能不能优化一下这个函数，让它支持缓存？"
    },
    "history": [
      {
        "userInputMessage": {
          "content": "请帮我写一个Python函数来计算斐波那契数列"
        }
      },
      {
        "assistantResponseMessage": {
          "content": "以下是一个计算斐波那契数列的Python函数：\n\n```python\ndef fibonacci(n):\n    if n <= 0:\n        return 0\n    elif n == 1:\n        return 1\n    else:\n        a, b = 0, 1\n        for _ in range(2, n + 1):\n            a, b = b, a + b\n        return b\n```"
        }
      }
    ]
  }
}
```

### 响应示例

```json
{
  "conversationId": "conv-002",
  "messages": [
    {
      "assistantResponseMessage": {
        "content": "好的，我来为您添加缓存功能来优化斐波那契函数：\n\n```python\nfrom functools import lru_cache\n\n@lru_cache(maxsize=None)\ndef fibonacci(n):\n    \"\"\"\n    计算斐波那契数列的第n项（带缓存优化）\n    \"\"\"\n    if n <= 0:\n        return 0\n    elif n == 1:\n        return 1\n    else:\n        return fibonacci(n-1) + fibonacci(n-2)\n\n# 示例用法\nprint(fibonacci(10))  # 输出：55\nprint(fibonacci(50))  # 因为有缓存，计算很快\n```\n\n使用缓存后，重复计算相同值时会直接从缓存中读取，大大提高了性能。"
      }
    }
  ],
  "usage": {
    "inputTokens": 25,
    "outputTokens": 180,
    "totalTokens": 205
  }
}
```

## 场景 3：带工具调用的单轮对话

### 请求示例（定义工具）

```json
{
  "modelId": "amazon.codewhisperer",
  "profileArn": "arn:aws:codewhisperer:us-east-1:123456789012:profile/user-profile",
  "conversationState": {
    "chatTriggerType": "TEXT",
    "conversationId": "conv-003",
    "currentMessage": {
      "content": "请帮我查询北京的天气情况",
      "userInputMessageContext": {
        "tools": [
          {
            "toolSpecification": {
              "name": "get_weather",
              "description": "获取指定城市的天气信息",
              "inputSchema": {
                "type": "object",
                "properties": {
                  "city": {
                    "type": "string",
                    "description": "城市名称"
                  },
                  "unit": {
                    "type": "string",
                    "enum": ["celsius", "fahrenheit"],
                    "description": "温度单位"
                  }
                },
                "required": ["city"]
              }
            }
          }
        ]
      }
    }
  }
}
```

### 响应示例（工具调用请求）

```json
{
  "conversationId": "conv-003",
  "messages": [
    {
      "assistantResponseMessage": {
        "content": "我将为您查询北京的天气信息。",
        "toolCalls": [
          {
            "toolCallId": "tool-001",
            "name": "get_weather",
            "input": {
              "city": "北京",
              "unit": "celsius"
            }
          }
        ]
      }
    }
  ],
  "usage": {
    "inputTokens": 35,
    "outputTokens": 45,
    "totalTokens": 80
  }
}
```

### 第二轮请求（提供工具执行结果）

```json
{
  "modelId": "amazon.codewhisperer",
  "profileArn": "arn:aws:codewhisperer:us-east-1:123456789012:profile/user-profile",
  "conversationState": {
    "chatTriggerType": "TEXT",
    "conversationId": "conv-003",
    "currentMessage": {
      "content": "工具执行结果如下",
      "userInputMessageContext": {
        "toolResults": [
          {
            "toolCallId": "tool-001",
            "content": [
              {
                "json": {
                  "city": "北京",
                  "temperature": 25,
                  "unit": "celsius",
                  "condition": "晴朗",
                  "humidity": 45,
                  "windSpeed": 12
                }
              }
            ]
          }
        ]
      }
    },
    "history": [
      {
        "userInputMessage": {
          "content": "请帮我查询北京的天气情况",
          "userInputMessageContext": {
            "tools": [
              {
                "toolSpecification": {
                  "name": "get_weather",
                  "description": "获取指定城市的天气信息",
                  "inputSchema": {
                    "type": "object",
                    "properties": {
                      "city": {
                        "type": "string",
                        "description": "城市名称"
                      },
                      "unit": {
                        "type": "string",
                        "enum": ["celsius", "fahrenheit"],
                        "description": "温度单位"
                      }
                    },
                    "required": ["city"]
                  }
                }
              }
            ]
          }
        }
      },
      {
        "assistantResponseMessage": {
          "content": "我将为您查询北京的天气信息。",
          "toolCalls": [
            {
              "toolCallId": "tool-001",
              "name": "get_weather",
              "input": {
                "city": "北京",
                "unit": "celsius"
              }
            }
          ]
        }
      }
    ]
  }
}
```

### 第二轮响应（基于工具结果的回答）

```json
{
  "conversationId": "conv-003",
  "messages": [
    {
      "assistantResponseMessage": {
        "content": "根据查询结果，北京当前的天气情况如下：\n\n- 温度：25°C\n- 天气状况：晴朗\n- 湿度：45%\n- 风速：12公里/小时\n\n今天北京的天气非常宜人，适合户外活动。"
      }
    }
  ],
  "usage": {
    "inputTokens": 50,
    "outputTokens": 80,
    "totalTokens": 130
  }
}
```

## 场景 4：带工具调用的多轮复杂对话

### 请求示例（多工具调用）

```json
{
  "modelId": "amazon.codewhisperer",
  "profileArn": "arn:aws:codewhisperer:us-east-1:123456789012:profile/user-profile",
  "conversationState": {
    "chatTriggerType": "TEXT",
    "conversationId": "conv-004",
    "currentMessage": {
      "content": "帮我规划一个周末的北京旅游行程，需要包含天气信息和景点推荐",
      "userInputMessageContext": {
        "tools": [
          {
            "toolSpecification": {
              "name": "get_weather",
              "description": "获取指定城市的天气信息",
              "inputSchema": {
                "type": "object",
                "properties": {
                  "city": {
                    "type": "string",
                    "description": "城市名称"
                  },
                  "date": {
                    "type": "string",
                    "format": "date",
                    "description": "查询日期"
                  }
                },
                "required": ["city"]
              }
            }
          },
          {
            "toolSpecification": {
              "name": "search_attractions",
              "description": "搜索旅游景点信息",
              "inputSchema": {
                "type": "object",
                "properties": {
                  "city": {
                    "type": "string",
                    "description": "城市名称"
                  },
                  "category": {
                    "type": "string",
                    "enum": ["historical", "cultural", "natural", "entertainment"],
                    "description": "景点类别"
                  },
                  "limit": {
                    "type": "integer",
                    "minimum": 1,
                    "maximum": 10,
                    "description": "返回结果数量限制"
                  }
                },
                "required": ["city"]
              }
            }
          }
        ]
      }
    }
  }
}
```

### 响应示例（多工具调用）

```json
{
  "conversationId": "conv-004",
  "messages": [
    {
      "assistantResponseMessage": {
        "content": "我将为您规划一个北京周末旅游行程。首先让我获取天气信息和景点推荐。",
        "toolCalls": [
          {
            "toolCallId": "tool-001",
            "name": "get_weather",
            "input": {
              "city": "北京",
              "date": "2024-06-15"
            }
          },
          {
            "toolCallId": "tool-002",
            "name": "search_attractions",
            "input": {
              "city": "北京",
              "category": "historical",
              "limit": 5
            }
          },
          {
            "toolCallId": "tool-003",
            "name": "search_attractions",
            "input": {
              "city": "北京",
              "category": "cultural",
              "limit": 3
            }
          }
        ]
      }
    }
  ],
  "usage": {
    "inputTokens": 65,
    "outputTokens": 75,
    "totalTokens": 140
  }
}
```

## 场景 5：流式响应处理

### 流式响应格式（逐帧）

```json
// 第一帧：开始响应
{
  "conversationId": "conv-005",
  "messages": [
    {
      "assistantResponseMessage": {
        "content": "",
        "toolCalls": []
      }
    }
  ]
}

// 第二帧：内容开始
{
  "conversationId": "conv-005",
  "messages": [
    {
      "assistantResponseMessage": {
        "content": "我将",
        "toolCalls": []
      }
    }
  ]
}

// 第三帧：内容继续
{
  "conversationId": "conv-005",
  "messages": [
    {
      "assistantResponseMessage": {
        "content": "我将为您",
        "toolCalls": []
      }
    }
  ]
}

// 最后一帧：完整响应+使用统计
{
  "conversationId": "conv-005",
  "messages": [
    {
      "assistantResponseMessage": {
        "content": "我将为您查询北京的天气信息。",
        "toolCalls": [
          {
            "toolCallId": "tool-001",
            "name": "get_weather",
            "input": {
              "city": "北京",
              "unit": "celsius"
            }
          }
        ]
      }
    }
  ],
  "usage": {
    "inputTokens": 15,
    "outputTokens": 45,
    "totalTokens": 60
  }
}
```

## 场景 6：错误处理

### 认证错误响应

```json
{
  "conversationId": "conv-006",
  "error": {
    "code": "AUTHENTICATION_ERROR",
    "message": "Invalid access token. Please refresh your credentials."
  }
}
```

### 频率限制错误响应

```json
{
  "conversationId": "conv-007",
  "error": {
    "code": "RATE_LIMIT_EXCEEDED",
    "message": "Rate limit exceeded. Please try again in 60 seconds."
  }
}
```

### 工具调用错误响应

```json
{
  "conversationId": "conv-008",
  "messages": [
    {
      "assistantResponseMessage": {
        "content": "抱歉，在执行工具调用时遇到了错误。",
        "toolCalls": []
      }
    }
  ],
  "error": {
    "code": "TOOL_EXECUTION_ERROR",
    "message": "Weather service is currently unavailable"
  }
}
```

## 技术约束和最佳实践

### JSON Schema 规范
- 必须过滤掉 Kiro API 不支持的字段（default、additionalProperties 等）
- 输入模式必须使用标准的 JSON Schema 格式
- 枚举值必须明确列出所有可能的值

### 对话历史格式
- 多轮对话的历史消息必须以 AssistantResponseMessage 结尾
- 工具调用结果必须包含完整的 toolCallId 匹配
- 历史消息的顺序必须保持对话的时间顺序

### 流式响应处理
- 必须正确处理 AWS 事件流的二进制格式
- 必须聚合工具调用的名称和输入参数
- 必须正确处理流式响应的结束标志

## 凭证格式规范

### Social 模式凭证
```json
{
  "accessToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refreshToken": "def50200d8a3e2c7a8b5c6d7e8f9a0b1...",
  "profileArn": "arn:aws:codewhisperer:us-east-1:123456789012:profile/user-profile",
  "expiresIn": 3600
}
```

### IDC 模式凭证
```json
{
  "clientId": "your-client-id",
  "clientSecret": "your-client-secret",
  "idcRegion": "us-east-1",
  "scope": "codewhisperer:chat"
}
```

## 成功标准

- 能够正确构建各种场景下的 Kiro 原生请求参数
- 能够解析和处理所有类型的 Kiro 原生响应格式
- 提供完整的错误处理和重试机制
- 支持流式和非流式两种响应模式
- 所有示例都基于实际的 AWS CodeWhisperer API 格式