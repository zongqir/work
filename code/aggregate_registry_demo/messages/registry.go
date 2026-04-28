package main

var bizAggregateResultRegistry = map[string]BizAggregateResultMeta{
	"xdr_risk_digest": {
		NewPayload: func() any {
			return &XdrRiskDigest{}
		},
	},
}
