package balance

import (
	"encoding/json"

	"github.com/sirupsen/logrus"
	"github.com/tema9984/AvitoInter/internal/model"
)

func JsonToUsersTrans(ubJn []byte) model.TransUser {
	ub := model.TransUser{}
	err := json.Unmarshal(ubJn, &ub)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"desc": "err := json.Unmarshal(ubJn, &ub) err",
			"func": "JsonToUserBal",
		}).Error(err)
		return model.TransUser{}
	}
	return ub
}
func JsonToUserBalance(ubJn []byte) model.UserBalance {
	ub := model.UserBalance{}
	err := json.Unmarshal(ubJn, &ub)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"desc": "err := json.Unmarshal(ubJn, &ub) err",
			"func": "JsonToUserBal",
		}).Error(err)
		return model.UserBalance{}
	}
	return ub
}
func HistoryToJson(history []model.History) ([]byte, error) {
	json, err := json.Marshal(history)
	return json, err
}
func userBalanceToJson(userBalance interface{}) ([]byte, error) {
	json, err := json.Marshal(userBalance)
	return json, err
}
func userBalanceConvToJson(userBalance interface{}) ([]byte, error) {
	json, err := json.Marshal(userBalance)
	return json, err
}
