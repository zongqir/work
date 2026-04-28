package messages

import (
	"fmt"
	"strconv"
)

var bizAggregateResultRegistry = map[string]BizAggregateResultMeta{
	"xdr_risk_digest": {
		NewPayload: func() any {
			return &XdrRiskDigest{}
		},
		BuildSMSParams: func(windowLabel string, payload any) (map[string]string, error) {
			digest, ok := payload.(*XdrRiskDigest)
			if !ok {
				return nil, fmt.Errorf("unexpected sms payload type: %T", payload)
			}

			return map[string]string{
				"window_label": windowLabel,
				"total_count":  strconv.Itoa(digest.TotalCount),
			}, nil
		},
	},
}

func LookupBizAggregateResultMeta(messageType string) (BizAggregateResultMeta, bool) {
	meta, ok := bizAggregateResultRegistry[messageType]
	return meta, ok
}
