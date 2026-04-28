package main

var registry = map[string]MessageSpec{
	"xdr_risk_digest": {
		TemplateCode: "xdr_risk_digest",
		NewPayload: func() any {
			return &XdrRiskDigest{}
		},
	},
}
