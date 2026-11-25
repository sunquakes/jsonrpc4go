package common

import (
	"encoding/json"
	"errors"
)

/**
 * @Description: Success response structure
 * @Field Id: Request ID
 * @Field JsonRpc: JSON-RPC version
 * @Field Result: Result
 */
type SuccessResponse struct {
	Id      any    `json:"id"`
	JsonRpc string `json:"jsonrpc"`
	Result  any    `json:"result"`
}

/**
 * @Description: Success notify response structure
 * @Field JsonRpc: JSON-RPC version
 * @Field Result: Result
 */
type SuccessNotifyResponse struct {
	JsonRpc string `json:"jsonrpc"`
	Result  any    `json:"result"`
}

/**
 * @Description: Error structure
 * @Field Code: Error code
 * @Field Message: Error message
 * @Field Data: Error data
 */
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

/**
 * @Description: Error response structure
 * @Field Id: Request ID
 * @Field JsonRpc: JSON-RPC version
 * @Field Error: Error information
 */
type ErrorResponse struct {
	Id      any    `json:"id"`
	JsonRpc string `json:"jsonrpc"`
	Error   Error  `json:"error"`
}

/**
 * @Description: Error notify response structure
 * @Field JsonRpc: JSON-RPC version
 * @Field Error: Error information
 */
type ErrorNotifyResponse struct {
	JsonRpc string `json:"jsonrpc"`
	Error   Error  `json:"error"`
}

/**
 * @Description: Create error response
 * @Param id: Request ID
 * @Param jsonRpc: JSON-RPC version
 * @Param errCode: Error code
 * @Return any: Error response structure
 */
func E(id any, jsonRpc string, errCode int) any {
	e := Error{
		errCode,
		CodeMap[errCode],
		nil,
	}
	var res any
	if id != nil {
		res = ErrorResponse{id, jsonRpc, e}
	} else {
		res = ErrorNotifyResponse{jsonRpc, e}
	}
	return res
}

/**
 * @Description: Create custom error response
 * @Param id: Request ID
 * @Param jsonRpc: JSON-RPC version
 * @Param errMessage: Error message
 * @Return any: Error response structure
 */
func CE(id any, jsonRpc string, errMessage string) any {
	e := Error{
		CustomError,
		errMessage,
		nil,
	}
	var res any
	if id != nil {
		res = ErrorResponse{id, jsonRpc, e}
	} else {
		res = ErrorNotifyResponse{jsonRpc, e}
	}
	return res
}

/**
 * @Description: Create success response
 * @Param id: Request ID
 * @Param jsonRpc: JSON-RPC version
 * @Param result: Result
 * @Return any: Success response structure
 */
func S(id any, jsonRpc string, result any) any {
	var res any
	if id != nil {
		res = SuccessResponse{id, jsonRpc, result}
	} else {
		res = SuccessNotifyResponse{jsonRpc, result}
	}
	return res
}

/**
 * @Description: Create JSON error response
 * @Param id: Request ID
 * @Param jsonRpc: JSON-RPC version
 * @Param errCode: Error code
 * @Return []byte: JSON error response data
 */
func jsonE(id any, jsonRpc string, errCode int) []byte {
	e, _ := json.Marshal(E(id, jsonRpc, errCode))
	return e
}

/**
 * @Description: Create a JSON error response envelope for HTTP transports
 * @Param id: Request ID
 * @Param errCode: Error code
 * @Return []byte: JSON error response data
 */
func JsonErrorEnvelope(id any, errCode int) []byte {
	return jsonE(id, JsonRpc, errCode)
}

/**
 * @Description: Batch item result structure
 * @Field Result: Result value decoded from the response
 * @Field Err: Error decoded from the response
 */
type ItemResult struct {
	Result any
	Err    error
}

/**
 * @Description: Get single response
 * @Param jsonData: JSON data
 * @Return any: Result value
 * @Return error: Error information
 */
func GetSingleResponse(jsonData map[string]any) (any, error) {
	emData, ok := jsonData["error"]
	if ok && emData != nil {
		resErr := new(Error)
		if err := GetStruct(emData, resErr); err != nil {
			return nil, err
		}
		Debug(resErr.Message)
		return nil, errors.New(resErr.Message)
	}
	return jsonData["result"], nil
}

/**
 * @Description: Get result
 * @Param b: Response data
 * @Return any: Result value
 * @Return error: Error information
 */
func GetResult(b []byte) (any, error) {
	var jsonData any
	if err := json.Unmarshal(b, &jsonData); err != nil {
		Debug(err)
		return nil, err
	}
	switch data := jsonData.(type) {
	case map[string]any:
		return GetSingleResponse(data)
	case []any:
		return nil, errors.New("json: unexpected batch response")
	}
	return nil, errors.New("json: invalid response")
}

/**
 * @Description: Get batch results
 * @Param b: Batch response data
 * @Return []ItemResult: Per-item results in request order
 * @Return error: Error information
 */
func GetBatchResult(b []byte) ([]ItemResult, error) {
	var jsonData any
	if err := json.Unmarshal(b, &jsonData); err != nil {
		Debug(err)
		return nil, err
	}
	data, ok := jsonData.([]any)
	if !ok {
		return nil, errors.New("json: batch response must be an array")
	}
	results := make([]ItemResult, 0, len(data))
	for _, v := range data {
		m, ok := v.(map[string]any)
		if !ok {
			results = append(results, ItemResult{Err: errors.New("json: invalid batch response item")})
			continue
		}
		result, err := GetSingleResponse(m)
		results = append(results, ItemResult{Result: result, Err: err})
	}
	return results, nil
}

/**
 * @Description: Parse response body
 * @Param b: Response data
 * @Return any: Parsed data
 * @Return error: Error information
 */
func ParseResponseBody(b []byte) (any, error) {
	var err error
	var jsonData any
	err = json.Unmarshal(b, &jsonData)
	if err != nil {
		Debug(err)
	}
	return jsonData, err
}
