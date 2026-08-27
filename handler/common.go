package handler

type Resp struct {
	Code int32       `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

func BadResp(msg string) *Resp {
	return &Resp{
		Code: -1,
		Msg:  msg,
	}
}
