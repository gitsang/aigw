package translate

import (
	"crypto/rand"
	"encoding/hex"
)

func randomSuffix() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "fallback000000000000"
	}
	return hex.EncodeToString(b)
}

func randomID(apiType string) string {
	prefix := "chatcmpl-"
	switch apiType {
	case APIResponses:
		prefix = "resp_"
	case APIMessages:
		prefix = "msg_"
	case APIInteractions:
		prefix = "v1_"
	}
	return prefix + randomSuffix()
}
