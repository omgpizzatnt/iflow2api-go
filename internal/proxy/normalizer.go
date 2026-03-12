package proxy

import "iflow2api-go/internal/logger"

func NormalizeResponse(result map[string]interface{}, preserveReasoning bool) map[string]interface{} {
	choices, ok := result["choices"].([]interface{})
	if !ok {
		return result
	}

	for _, choice := range choices {
		choiceMap, ok := choice.(map[string]interface{})
		if !ok {
			continue
		}

		message, ok := choiceMap["message"].(map[string]interface{})
		if !ok {
			continue
		}

		content, _ := message["content"].(string)
		reasoningContent, hasReasoning := message["reasoning_content"].(string)

		if content == "" && hasReasoning && reasoningContent != "" {
			if preserveReasoning {
				delete(message, "reasoning_content")
				message["content"] = reasoningContent
				logger.Debugf("Preserve reasoning: reasoning_content (len=%d) -> content", len(reasoningContent))
			} else {
				delete(message, "reasoning_content")
				message["content"] = reasoningContent
				logger.Debugf("Merge reasoning: reasoning_content -> content (len=%d)", len(reasoningContent))
			}
		} else if content != "" && hasReasoning && reasoningContent != "" {
			if !preserveReasoning {
				logger.Debugf("Merge reasoning: remove reasoning_content, keep content (len=%d)", len(content))
				delete(message, "reasoning_content")
			} else {
				logger.Debugf("Response has content (len=%d) and reasoning_content (len=%d)", len(content), len(reasoningContent))
			}
		} else if content == "" && (!hasReasoning || reasoningContent == "") {
			logger.Warn("content and reasoning_content both empty in message")
		}
	}

	return result
}

func NormalizeStreamChunk(chunkData map[string]interface{}, preserveReasoning bool) map[string]interface{} {
	choices, ok := chunkData["choices"].([]interface{})
	if !ok {
		return chunkData
	}

	for _, choice := range choices {
		choiceMap, ok := choice.(map[string]interface{})
		if !ok {
			continue
		}

		delta, ok := choiceMap["delta"].(map[string]interface{})
		if !ok {
			continue
		}

		content, _ := delta["content"].(string)
		reasoningContent, hasReasoning := delta["reasoning_content"].(string)

		if content == "" && hasReasoning && reasoningContent != "" {
			if !preserveReasoning {
				delta["content"] = reasoningContent
				delete(delta, "reasoning_content")
			}
		} else if content != "" && hasReasoning && reasoningContent != "" {
			if content == reasoningContent {
				delete(delta, "reasoning_content")
			} else if !preserveReasoning {
				delete(delta, "reasoning_content")
			}
		}
	}

	return chunkData
}
