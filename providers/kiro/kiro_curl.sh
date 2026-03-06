#!/bin/bash

# 设置请求参数
ACCESS_TOKEN="your_access_token_here"
REGION="us-east-1"
MODEL="claude-sonnet-4-5"
CONVERSATION_ID=$(uuidgen)

# 构建请求URL
BASE_URL="https://q.${REGION}.amazonaws.com/generateAssistantResponse"

# 构建请求头
HEADERS=(
    "-H" "Authorization: Bearer ${ACCESS_TOKEN}"
    "-H" "amz-sdk-invocation-id: $(uuidgen)"
    "-H" "Content-Type: application/json"
    "-H" "Accept: application/json"
)

# 构建带工具规格的请求体
REQUEST_BODY=$(cat << EOF
{
  "conversationState": {
    "chatTriggerType": "MANUAL",
    "conversationId": "${CONVERSATION_ID}",
    "history": [
      {
        "userInputMessage": {
          "content": "请帮我查询今天的天气信息",
          "modelId": "CLAUDE_SONNET_4_5_20250929_V1_0",
          "origin": "AI_EDITOR",
          "userInputMessageContext": {
            "tools": [
              {
                "toolSpecification": {
                  "name": "get_weather",
                  "description": "查询指定城市的天气信息，包括温度、湿度、天气状况等",
                  "inputSchema": {
                    "json": {
                      "type": "object",
                      "properties": {
                        "city": {
                          "type": "string",
                          "description": "城市名称"
                        },
                        "date": {
                          "type": "string",
                          "description": "日期，格式为YYYY-MM-DD"
                        }
                      },
                      "required": ["city"]
                    }
                  }
                }
              },
              {
                "toolSpecification": {
                  "name": "search_web",
                  "description": "在互联网上搜索相关信息",
                  "inputSchema": {
                    "json": {
                      "type": "object",
                      "properties": {
                        "query": {
                          "type": "string",
                          "description": "搜索关键词"
                        }
                      },
                      "required": ["query"]
                    }
                  }
                }
              }
            ]
          }
        }
      },
      {
        "assistantResponseMessage": {
          "content": "我将使用天气查询工具来获取今天的天气信息。",
          "toolUses": [
            {
              "toolUseId": "tool-001",
              "name": "get_weather",
              "input": "{\"city\": \"北京\", \"date\": \"2026-03-05\"}"
            }
          ]
        }
      }
    ],
    "currentMessage": {
      "userInputMessage": {
        "content": "请继续查询上海的天气",
        "modelId": "CLAUDE_SONNET_4_5_20250929_V1_0",
        "origin": "AI_EDITOR",
        "userInputMessageContext": {
          "toolResults": [
            {
              "toolUseId": "tool-001",
              "content": [
                {
                  "text": "{\"city\": \"北京\", \"date\": \"2026-03-05\", \"temperature\": \"15°C\", \"humidity\": \"45%\", \"condition\": \"晴\"}"
                }
              ],
              "status": "success"
            }
          ],
          "tools": [
            {
              "toolSpecification": {
                "name": "get_weather",
                "description": "查询指定城市的天气信息，包括温度、湿度、天气状况等",
                "inputSchema": {
                  "json": {
                    "type": "object",
                    "properties": {
                      "city": {
                        "type": "string",
                        "description": "城市名称"
                      },
                      "date": {
                        "type": "string",
                        "description": "日期，格式为YYYY-MM-DD"
                      }
                    },
                    "required": ["city"]
                  }
                }
              }
            }
          ]
        }
      }
    }
  }
}
EOF
)

-- response
{
  "assistantResponse": {
    "content": "我将为您查询上海的天气信息。",
    "toolUses": [
      {
        "toolUseId": "tool-002",
        "name": "get_weather",
        "input": "{\"city\": \"上海\", \"date\": \"2026-03-05\"}"
      }
    ]
  },
  "conversationId": "${CONVERSATION_ID}",
  "usage": {
    "inputTokens": 312,
    "outputTokens": 45,
    "totalTokens": 357
  }
}