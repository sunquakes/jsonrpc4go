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
	Id      string `json:"id"`
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
	Id      string `json:"id"`
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
		res = ErrorResponse{id.(string), jsonRpc, e}
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
		res = ErrorResponse{id.(string), jsonRpc, e}
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
		res = SuccessResponse{id.(string), jsonRpc, result}
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
 * @Description: Get single response
 * @Param jsonData: JSON data
 * @Param result: Result
 * @Return error: Error information
 */
func GetSingleResponse(jsonData map[string]any, result any) error {
	var (
		err error
	)
	emData, ok := jsonData["error"]
	if ok && emData != nil {
		resErr := new(Error)
		if err := GetStruct(emData, resErr); err != nil {
			return err
		}
		Debug(resErr.Message)
		return errors.New(resErr.Message)
	}
	// Notifications have no result to decode.
	if result == nil {
		return nil
	}
	jsonStr, err := json.Marshal(jsonData["result"])
	if err != nil {
		Debug(err)
		return err
	}
	err = json.Unmarshal(jsonStr, result)
	if err != nil {
		Debug(err)
		return err
	}
	return err
}

/**
 * @Description: Get result
 * @Param b: Response data
 * @Param result: Result
 * @Return error: Error information
 */
func GetResult(b []byte, result any) error {
	var jsonData any
	if err := json.Unmarshal(b, &jsonData); err != nil {
		Debug(err)
		return err
	}
	switch data := jsonData.(type) {
	case map[string]any:
		return GetSingleResponse(data, result)
	case []any:
		results, ok := result.([]*SingleRequest)
		if !ok {
			return errors.New("json: batch result must be a []*SingleRequest")
		}
		if len(results) != len(data) {
			return errors.New("json: batch response length does not match request length")
		}
		for k, v := range data {
			m, ok := v.(map[string]any)
			if !ok {
				continue
			}
			if err := GetSingleResponse(m, results[k].Result); err != nil {
				if results[k].Error != nil {
					*results[k].Error = err
				}
			}
		}
		return nil
	}
	return nil
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
