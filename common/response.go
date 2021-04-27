package common

import (
	"encoding/json"
	"errors"
	"github.com/goinggo/mapstructure"
	"reflect"
)

type SuccessResponse struct {
	Id      string      `json:"id"`
	JsonRpc string      `json:"jsonrpc"`
	Result  interface{} `json:"result"`
}

type SuccessNotifyResponse struct {
	JsonRpc string      `json:"jsonrpc"`
	Result  interface{} `json:"result"`
}

type Error struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
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

func E(id interface{}, jsonRpc string, errCode int) interface{} {
	e := Error{
		errCode,
		CodeMap[errCode],
		nil,
	}
	var res interface{}
	if id != nil {
		res = ErrorResponse{id.(string), jsonRpc, e}
	} else {
		res = ErrorNotifyResponse{jsonRpc, e}
	}
	return res
}

func CE(id interface{}, jsonRpc string, errMessage string) interface{} {
	e := Error{
		CustomError,
		errMessage,
		nil,
	}
	var res interface{}
	if id != nil {
		res = ErrorResponse{id.(string), jsonRpc, e}
	} else {
		res = ErrorNotifyResponse{jsonRpc, e}
	}
	return res
}

func S(id interface{}, jsonRpc string, result interface{}) interface{} {
	var res interface{}
	if id != nil {
		res = SuccessResponse{id.(string), jsonRpc, result}
	} else {
		res = SuccessNotifyResponse{jsonRpc, result}
	}
	return res
}

func jsonE(id interface{}, jsonRpc string, errCode int) []byte {
	e, _ := json.Marshal(E(id, jsonRpc, errCode))
	return e
}

func jsonS(id interface{}, jsonRpc string, result interface{}) []byte {
	s, _ := json.Marshal(S(id, jsonRpc, result))
	return s
}

func GetSingleResponse(jsonData map[string]interface{}, result interface{}) error {
	var (
		err    error
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

func GetResult(b []byte, result interface{}) error {
	var (
		err      error
		jsonData interface{}
	)
	err = json.Unmarshal(b, &jsonData)
	if err != nil {
		Debug(err)
	}
	if reflect.ValueOf(jsonData).Kind() == reflect.Map {
		err = GetSingleResponse(jsonData.(map[string]interface{}), result)
		if err != nil {
			return err
		}
	} else if reflect.ValueOf(jsonData).Kind() == reflect.Slice {
		for k, v := range jsonData.([]interface{}) {
			err = GetSingleResponse(v.(map[string]interface{}), (result.([]*SingleRequest)[k].Result))
			if err != nil {
				*(result.([]*SingleRequest)[k].Error) = err
			}
		}
	}
	return nil
}

func ParseResponseBody(b []byte) (interface{}, error) {
	var err error
	var jsonData interface{}
	err = json.Unmarshal(b, &jsonData)
	if err != nil {
		Debug(err)
	}
	return jsonData, err
}
