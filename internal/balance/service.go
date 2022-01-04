package balance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
	"github.com/tema9984/AvitoInter/config"
	"github.com/tema9984/AvitoInter/internal/model"
	"github.com/tema9984/AvitoInter/internal/store"
)

const (
	OperationsOnPage = 50
)

var ctx = context.Background()

type Service interface {
	ConfigureService()
	HandlerTransaction(w http.ResponseWriter, r *http.Request)
	HandlerReplenish(w http.ResponseWriter, r *http.Request)
	HandlerGetBalance(w http.ResponseWriter, r *http.Request)
	HandlerHistory(w http.ResponseWriter, r *http.Request)
	Logging(next http.Handler) http.Handler
}

type service struct {
	conf   *config.Config
	store  *store.Store
	logger *logrus.Logger
	rdb    *redis.Client
}

// NewService ...
func NewService(cfg *config.Config) Service {
	svc := &service{conf: cfg}
	return svc
}
func (svc *service) ConfigureService() {
	err := svc.configureStore()
	go svc.addToReadis()
	svc.logger = logrus.New()
	err = svc.configureRedis()
	if err != nil {
		logrus.Error(err)
	}
}
func (svc *service) addToReadis() {
	cache := svc.store.Ev().AllUser()
	for k, v := range cache {
		err := svc.rdb.Set(ctx, k, v.Balance, 0).Err()
		if err != nil {
			logrus.Warn(err)
		}
	}
}
func (svc *service) configureStore() error {
	st := store.New(svc.conf)
	if err := st.Open(); err != nil {
		logrus.Error("ERROR IN OPEN DB")
		return err
	}
	svc.store = st
	return nil
}
func (svc *service) configureRedis() error {
	svc.rdb = redis.NewClient(&redis.Options{
		Addr:     svc.conf.RedisAddr,
		Password: svc.conf.RedisPass,
		DB:       svc.conf.RedisDB,
	})
	return nil
}

//Обработчик транзакций от одного юзера к другому
func (svc *service) HandlerTransaction(w http.ResponseWriter, r *http.Request) {
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"desc": "ioutil.ReadAll(r.Body) err",
			"func": "handlerTransaction",
		}).Error(err)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}
	modTr := JsonToUsersTrans(b)
	if modTr.Amount < 0 {
		err = fmt.Errorf("value less than 0, only values greater than 0")
	}
	if modTr.User1ID == modTr.User2ID {
		err = fmt.Errorf("the transaction does not make sense")
	}
	user1, exist1 := svc.GetBalance(modTr.User1ID)
	if !exist1 {
		err = fmt.Errorf("User 1 not found")
	}
	if user1.Balance < modTr.Amount {
		err = fmt.Errorf("The balance is less than the debit operation")
	}
	user2, exist2 := svc.GetBalance(modTr.User2ID)
	if !exist2 {
		err = fmt.Errorf("User 2 not found")
	}
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"desc": "svc.GetBalance",
			"func": "handlerTransaction",
		}).Warn(err)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}
	err = svc.store.Ev().Transaction(user1, user2, modTr.Amount)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"desc": "svc.store.Ev().Transaction(user1, user2, modTr.Amount) err",
			"func": "handlerTransaction",
		}).Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	err = svc.rdb.Set(ctx, user1.UserID, user1.Balance-modTr.Amount, 0).Err()
	if err != nil {
		logrus.Error(err)
	}
	err = svc.rdb.Set(ctx, user2.UserID, user2.Balance+modTr.Amount, 0).Err()
	if err != nil {
		logrus.Error(err)
	}
}

//Обработчик пополнения и списания средств у юзера
func (svc *service) HandlerReplenish(w http.ResponseWriter, r *http.Request) {
	var err error
	defer func() {
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
		}
	}()
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"desc": "ioutil.ReadAll(r.Body) err",
			"func": "handlerTransaction",
		}).Error(err)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}
	isDebit := false
	req := JsonToUserBalance(b)
	if req.Balance < 0 {
		isDebit = true
	}
	userBal, exist := svc.GetBalance(req.UserID)
	if isDebit && userBal.Balance < req.Balance*-1 {
		if !exist {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("User not found"))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("The balance is less than the debit operation"))
		return
	}
	if !exist && !isDebit {
		err = svc.store.Ev().CreateUser(req)
	} else {
		amount := req.Balance
		req.Balance += userBal.Balance
		err = svc.store.Ev().Replenishment(req, amount)
	}
	if err != nil {
		return
	}
	err = svc.rdb.Set(ctx, req.UserID, req.Balance, 0).Err()
	if err != nil {
		logrus.Error(err)
	}
}

//Обработчик истории операций
func (svc *service) HandlerHistory(w http.ResponseWriter, r *http.Request) {
	var err error
	defer func() {
		if err != nil {
			w.Write([]byte(err.Error()))
		}
	}()
	id, ok := r.URL.Query()["id"]
	sortType, isSort := r.URL.Query()["sort"]
	column, isColumn := r.URL.Query()["column"]
	pageStr, isPage := r.URL.Query()["page"]
	page := 1
	if isPage {
		page, err = strconv.Atoi(pageStr[0])
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}
	st := false
	if isSort {
		if sortType[0] == "desc" {
			st = true
		}
	}
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		err = errors.New("incorrect query string")
		return
	}
	history := svc.store.Ev().HistoryByID(id[0])
	if isColumn {
		SortHistory(history, column[0], st)
	}
	start := OperationsOnPage * (page - 1)
	end := OperationsOnPage * page
	if OperationsOnPage*page > len(history) {
		end = len(history)
	}
	if start > len(history) {
		return
	}
	json, err := HistoryToJson(history[start:end])
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
	w.Write(json)
}

//Обработчик для получения баланса юзера
func (svc *service) HandlerGetBalance(w http.ResponseWriter, r *http.Request) {
	var err error
	defer func() {
		if err != nil {
			w.Write([]byte(err.Error()))
		}
	}()
	id, ok := r.URL.Query()["id"]
	currency, isConvert := r.URL.Query()["currency"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		err = errors.New("incorrect query string")
		return
	}
	userID := id[0]
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	user, exist := svc.GetBalance(userID)
	if !exist {
		w.WriteHeader(http.StatusBadRequest)
		err = fmt.Errorf("User not found")
		return
	}
	resp := []byte{}
	if isConvert {
		userConv := model.UserBalanceConv{UserID: user.UserID}
		userConv.Balance = svc.convert(currency[0], user.Balance)
		resp, err = userBalanceToJson(userConv)
	} else {
		resp, err = userBalanceToJson(user)
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write(resp)

}

// функция для сортировки массива историй операций
func SortHistory(history []model.History, typeSort string, desc bool) {
	if typeSort == "date" {
		sort.Slice(history, func(i, j int) bool {
			more := false
			if history[i].Date.After(history[j].Date) {
				more = true
			} else {
				more = false
			}
			if desc {
				return more
			} else {
				return !more
			}

		})
	}
	if typeSort == "amount" {
		sort.Slice(history, func(i, j int) bool {
			more := false
			if history[i].Amount > history[j].Amount {
				more = true
			} else {
				more = false
			}
			if desc {
				return more
			} else {
				return !more
			}

		})
	}
}

// функция для конвертации рублей в другую валюту
func (svc *service) convert(currency string, balance int) float64 {
	convert, err := http.Get("https://freecurrencyapi.net/api/v2/latest?apikey=" + svc.conf.ApiKey + "&base_currency=" + currency)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"desc": `convert, err := http.Get("https://freecurrencyapi.net/api/v2/latest?apikey=` + svc.conf.ApiKey + `&base_currency=` + currency + `)`,
			"func": "convert",
		}).Error(err)
		return 0
	}
	cur := model.Currency{}
	b, err := ioutil.ReadAll(convert.Body)
	err = json.Unmarshal(b, &cur)
	return float64(balance) / cur.Rub.Rub
}

//Функция для получения баланса из кеша
func (svc *service) GetBalance(id string) (model.UserBalance, bool) {
	val, err := svc.rdb.Get(ctx, id).Result()
	if err == redis.Nil {
		return model.UserBalance{}, false
	} else if err != nil {
		logrus.Error(err)
		return model.UserBalance{}, false
	}
	balance, err := strconv.Atoi(val)
	if err != nil {
		logrus.Error(err)
		return model.UserBalance{}, false
	}
	userBal, exist := model.UserBalance{UserID: id, Balance: balance}, true
	return userBal, exist
}

func (svc *service) Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, req)
		svc.logger.Printf("%s %s %s", req.Method, req.RequestURI, time.Since(start))
	})
}
