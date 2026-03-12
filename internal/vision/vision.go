// Package vision provides vision and image processing utilities for iFlow2API.
package vision

import (
	"encoding/base64"
	"fmt"
	"strings"
)

var visionModels = map[string]VisionModelInfo{
	"qwen-vl-max": {
		Name:      "Qwen-VL-Max",
		Provider:  "alibaba",
		MaxImages: 10,
	},
}

const defaultVisionModel = "qwen-vl-max"

type VisionModelInfo struct {
	Name      string
	Provider  string
	MaxImages int
}

type ImageData struct {
	Data      string
	IsURL     bool
	MediaType string
	Detail    string
}

func IsVisionModel(model string) bool {
	_, ok := visionModels[strings.ToLower(model)]
	return ok
}

func GetVisionModelInfo(model string) *VisionModelInfo {
	info, ok := visionModels[strings.ToLower(model)]
	if !ok {
		return nil
	}
	return &info
}

func SupportsVision(model string) bool {
	return IsVisionModel(model)
}

func GetMaxImages(model string) int {
	info := GetVisionModelInfo(model)
	if info == nil {
		return 0
	}
	return info.MaxImages
}

func DetectImageContent(content interface{}) []ImageData {
	images := []ImageData{}

	contentList, ok := content.([]interface{})
	if !ok {
		return images
	}

	for _, block := range contentList {
		blockMap, ok := block.(map[string]interface{})
		if !ok {
			continue
		}

		blockType, _ := blockMap["type"].(string)

		if blockType == "image_url" {
			imageURL := blockMap["image_url"]
			var url string

			if urlMap, ok := imageURL.(map[string]interface{}); ok {
				url, _ = urlMap["url"].(string)
			} else {
				url, _ = imageURL.(string)
			}

			if url != "" {
				if strings.HasPrefix(url, "data:") {
					if mediaType, data, err := parseDataURL(url); err == nil {
						images = append(images, ImageData{
							Data:      data,
							IsURL:     false,
							MediaType: mediaType,
							Detail:    "auto",
						})
					}
				} else {
					images = append(images, ImageData{
						Data:   url,
						IsURL:  true,
						Detail: "auto",
					})
				}
			}
		} else if blockType == "image" {
			source, _ := blockMap["source"].(map[string]interface{})
			if sourceType, _ := source["type"].(string); sourceType == "base64" {
				images = append(images, ImageData{
					Data:      source["data"].(string),
					IsURL:     false,
					MediaType: "image/png",
				})
			} else if sourceType == "url" {
				images = append(images, ImageData{
					Data:  source["url"].(string),
					IsURL: true,
				})
			}
		}
	}

	return images
}

func parseDataURL(dataURL string) (string, string, error) {
	if !strings.HasPrefix(dataURL, "data:") {
		return "", "", fmt.Errorf("not a data URL")
	}

	rest := dataURL[5:]

	commaIdx := strings.Index(rest, ",")
	if commaIdx == -1 {
		return "", "", fmt.Errorf("invalid data URL: missing comma")
	}

	mediaTypePart := rest[:commaIdx]
	dataPart := rest[commaIdx+1:]

	mediaType := strings.Replace(mediaTypePart, ";base64", "", 1)
	if mediaType == "" {
		mediaType = "image/png"
	}

	return mediaType, dataPart, nil
}

func ConvertToOpenAIFormat(images []ImageData) []map[string]interface{} {
	blocks := []map[string]interface{}{}

	for _, img := range images {
		if img.IsURL {
			blocks = append(blocks, map[string]interface{}{
				"type": "image_url",
				"image_url": map[string]interface{}{
					"url":    img.Data,
					"detail": img.Detail,
				},
			})
		} else {
			dataURL := fmt.Sprintf("data:%s;base64,%s", img.MediaType, img.Data)
			blocks = append(blocks, map[string]interface{}{
				"type": "image_url",
				"image_url": map[string]interface{}{
					"url": dataURL,
				},
			})
		}
	}

	return blocks
}

func ConvertToAnthropicFormat(images []ImageData) []map[string]interface{} {
	blocks := []map[string]interface{}{}

	for _, img := range images {
		if img.IsURL {
			blocks = append(blocks, map[string]interface{}{
				"type": "image",
				"source": map[string]interface{}{
					"type": "url",
					"url":  img.Data,
				},
			})
		} else {
			blocks = append(blocks, map[string]interface{}{
				"type": "image",
				"source": map[string]interface{}{
					"type":       "base64",
					"media_type": img.MediaType,
					"data":       img.Data,
				},
			})
		}
	}

	return blocks
}

func ProcessMessageContent(content interface{}, targetFormat string) interface{} {
	contentList, ok := content.([]interface{})
	if !ok {
		return content
	}

	var textParts []string
	var images []ImageData

	for _, block := range contentList {
		if str, ok := block.(string); ok {
			textParts = append(textParts, str)
		} else if blockMap, ok := block.(map[string]interface{}); ok {
			blockType, _ := blockMap["type"].(string)

			if blockType == "text" {
				textParts = append(textParts, blockMap["text"].(string))
			} else if blockType == "image_url" {
				imageURL := blockMap["image_url"]
				var url string
				var detail string

				if urlMap, ok := imageURL.(map[string]interface{}); ok {
					url, _ = urlMap["url"].(string)
					detail, _ = urlMap["detail"].(string)
				} else {
					url, _ = imageURL.(string)
				}

				if url != "" {
					if strings.HasPrefix(url, "data:") {
						if mediaType, data, err := parseDataURL(url); err == nil {
							images = append(images, ImageData{
								Data:      data,
								IsURL:     false,
								MediaType: mediaType,
								Detail:    detail,
							})
						}
					} else {
						images = append(images, ImageData{
							Data:   url,
							IsURL:  true,
							Detail: detail,
						})
					}
				}
			} else if blockType == "image" {
				source, _ := blockMap["source"].(map[string]interface{})
				sourceType, _ := source["type"].(string)

				if sourceType == "base64" {
					images = append(images, ImageData{
						Data:      source["data"].(string),
						IsURL:     false,
						MediaType: source["media_type"].(string),
					})
				} else if sourceType == "url" {
					images = append(images, ImageData{
						Data:  source["url"].(string),
						IsURL: true,
					})
				}
			}
		}
	}

	var newBlocks []interface{}

	if len(textParts) > 0 {
		combinedText := strings.Join(textParts, "\n")
		if strings.TrimSpace(combinedText) != "" {
			newBlocks = append(newBlocks, map[string]interface{}{
				"type": "text",
				"text": combinedText,
			})
		}
	}

	if targetFormat == "openai" {
		for _, block := range ConvertToOpenAIFormat(images) {
			newBlocks = append(newBlocks, block)
		}
	} else {
		for _, block := range ConvertToAnthropicFormat(images) {
			newBlocks = append(newBlocks, block)
		}
	}

	if len(newBlocks) > 0 {
		return newBlocks
	}

	return content
}

func GetVisionModelsList() []map[string]interface{} {
	models := []map[string]interface{}{}

	for modelID, info := range visionModels {
		models = append(models, map[string]interface{}{
			"id":              modelID,
			"name":            info.Name,
			"provider":        info.Provider,
			"max_images":      info.MaxImages,
			"supports_vision": true,
		})
	}

	return models
}

func EstimateImageTokens(image ImageData) int {
	if image.Detail == "low" {
		return 85
	}

	return 765
}

func ValidateImageData(data string, isURL bool) bool {
	if data == "" {
		return false
	}

	if isURL {
		decoded, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			return false
		}
		return len(decoded) >= 100
	}

	return strings.HasPrefix(data, "http://") || strings.HasPrefix(data, "https://") || strings.HasPrefix(data, "data:")
}
