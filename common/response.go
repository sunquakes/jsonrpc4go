package common

import (
	"encoding/json"
	"errors"
	"github.com/goinggo/mapstructure"
	"reflect"
)

type SuccessResponse struct {
	Id      string `json:"id"`
	JsonRpc string `json:"jsonrpc"`
	Result  any    `json:"result"`
}

type SuccessNotifyResponse struct {
	JsonRpc string `json:"jsonrpc"`
	Result  any    `json:"result"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

type ErrorResponse struct {
	Id      string `json:"id"`
	JsonRpc string `json:"jsonrpc"`
	Error   Error  `json:"error"`
}

type ErrorNotifyResponse struct {
	JsonRpc string `json:"jsonrpc"`
	Error   Error  `json:"error"`
}

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

func S(id any, jsonRpc string, result any) any {
	var res any
	if id != nil {
		res = SuccessResponse{id.(string), jsonRpc, result}
	} else {
		res = SuccessNotifyResponse{jsonRpc, result}
	}
	return res
}

func jsonE(id any, jsonRpc string, errCode int) []byte {
	e, _ := json.Marshal(E(id, jsonRpc, errCode))
	return e
}

func jsonS(id any, jsonRpc string, result any) []byte {
	s, _ := json.Marshal(S(id, jsonRpc, result))
	return s
}

func GetSingleResponse(jsonData map[string]any, result any) error {
	var (
		err error
	)
	emData, ok := jsonData["error"]
	if ok {
		resErr := new(Error)
		err = GetStruct(emData, resErr)
		Debug(resErr.Message)
		return errors.New(resErr.Message)
	}
	if err = mapstructure.Decode(jsonData["result"], result); err != nil {
		Debug(err)
		return err
	}
	return err
}

func GetResult(b []byte, result any) error {
	var (
		err      error
		jsonData any
	)
	err = json.Unmarshal(b, &jsonData)
	if err != nil {
		Debug(err)
	}
	if reflect.ValueOf(jsonData).Kind() == reflect.Map {
		err = GetSingleResponse(jsonData.(map[string]any), result)
		if err != nil {
			return err
		}
	} else if reflect.ValueOf(jsonData).Kind() == reflect.Slice {
		for k, v := range jsonData.([]any) {
			err = GetSingleResponse(v.(map[string]any), (result.([]*SingleRequest)[k].Result))
			if err != nil {
				*(result.([]*SingleRequest)[k].Error) = err
			}
		}
	}
	return nil
}

func ParseResponseBody(b []byte) (any, error) {
	var err error
	var jsonData any
	err = json.Unmarshal(b, &jsonData)
	if err != nil {
		Debug(err)
	}
	return jsonData, err
}
