//
// Copyright (c) 2018 SK TECHX.
// All right reserved.
//
// This software is the confidential and proprietary information of SK TECHX.
// You shall not disclose such Confidential Information and
// shall use it only in accordance with the terms of the license agreement
// you entered into with SK TECHX.
//
//
// @project grpczk
// @author 1100282
// @date 2018. 3. 9. PM 4:06
//

package grpczk

import (
	"context"
	"google.golang.org/grpc/metadata"
)

const (
	GrpcTransactionId = "grpczk-transaction-id"
)

func GetGrpcTransactionId(ctx context.Context) string {
	headers, ok := metadata.FromIncomingContext(ctx)
	if ok {
		transactionId, ok := headers[GrpcTransactionId]
		if ok && len(transactionId) > 0 {
			return transactionId[0]
		}
	}

	return ""
}
