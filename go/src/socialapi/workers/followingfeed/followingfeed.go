package main

import (
	"flag"
	"koding/tools/config"
	followingfeed "socialapi/workers/followingfeed/lib"
	"socialapi/workers/helper"

	"github.com/jinzhu/gorm"
	"github.com/koding/bongo"
	"github.com/koding/logging"
	"github.com/streadway/amqp"
)

var (
	Bongo       *bongo.Bongo
	log         logging.Logger
	conf        *config.Config
	flagProfile = flag.String("c", "", "Configuration profile from file")
	flagDebug   = flag.Bool("d", false, "Debug mode")
	handler     *followingfeed.FollowingFeedController
)

func main() {
	flag.Parse()
	if *flagProfile == "" {
		log.Fatal("Please define config file with -c", "")
	}

	conf = config.MustConfig(*flagProfile)

	rmqConf := &rabbitmq.Config{
		Host:     conf.Mq.Host,
		Port:     conf.Mq.Port,
		Username: conf.Mq.ComponentUser,
		Password: conf.Mq.Password,
		Vhost:    conf.Mq.Vhost,
	}
	// create logger for our package
	log = helper.CreateLogger("TopicFeedWorker", *flagDebug)

	initBongo(rmqConf)

	// blocking
	followingfeed.Listen(rabbitmq.New(rmqConf, log), startHandler)
	defer followingfeed.Consumer.Shutdown()
}

func startHandler() func(delivery amqp.Delivery) {
	log.Info("Worker Started to Consume")
	return func(delivery amqp.Delivery) {
		err := handler.HandleEvent(delivery.Type, delivery.Body)
		switch err {
		case nil:
			delivery.Ack(false)
		case followingfeed.HandlerNotFoundErr:
			log.Notice("unknown event type (%s) recieved, \n deleting message from RMQ", delivery.Type)
			delivery.Ack(false)
		case gorm.RecordNotFound:
			log.Warning("Record not found in our db (%s) recieved, \n deleting message from RMQ", string(delivery.Body))
			delivery.Ack(false)
		default:
			// add proper error handling
			// instead of puttting message back to same queue, it is better
			// to put it to another maintenance queue/exchange
			log.Error("an error occured %s, \n putting message back to queue", err)
			// multiple false
			// reque true
			delivery.Nack(false, true)
		}
	}
}

func initBongo(c *rabbitmq.Config) {
	bConf := &broker.Config{
		RMQConfig: c,
	}
	broker := broker.New(bConf, log)
	Bongo = bongo.New(broker, db.DB, log)
	Bongo.Connect()
}

