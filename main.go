package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/adlio/harvest"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sns"
	forex "github.com/g3kk0/go-forex"
	"github.com/jinzhu/now"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

func main() {
 	lambda.Start(HandleRequest)
}

type UnusedEvent struct {

}

const RoundDownTo = 0.25

func HandleRequest(ctx context.Context, name UnusedEvent) (string, error) {
	harvestAccountId := os.Getenv("HARVEST_ACCOUNT_ID")
	harvestAccessToken := os.Getenv("HARVEST_ACCESS_TOKEN")
	client := harvest.NewTokenAPI(harvestAccountId, harvestAccessToken)

	clientId := os.Getenv("HARVEST_CLIENT_ID")
	startOfMonth := now.BeginningOfMonth()
	endOfMonth := now.EndOfMonth()
	args := harvest.Arguments{
		"client_id": clientId,
	}
	timeEntries, timeErr := client.GetTimeEntriesBetween(startOfMonth, endOfMonth, args)

	if timeErr != nil {
		msg := fmt.Sprintf("Error occurred while getting time %v\n", timeErr)
		return "", errors.New(msg)
	}
	var totalTime float64 = 0
	for _, timeEntry := range timeEntries {
		fmt.Printf("Time entry %v\n", timeEntry.Hours)
		totalTime += math.Round(timeEntry.Hours/RoundDownTo) * RoundDownTo
	}
	currentDay := time.Now()
	hoursLeft := 0.0
	for {
		currentDay = currentDay.Add(24 * time.Hour)
		if currentDay.After(endOfMonth) {
			break
		}
		if currentDay.Weekday() == time.Sunday || currentDay.Weekday() == time.Saturday {
			continue
		}
		hoursLeft += 8.0
	}

	awsConfig := &aws.Config{
		Region:      aws.String(os.Getenv("AWS_REGION")),
	}
	awsId := os.Getenv("AWS_ID")
	awsKey := os.Getenv("AWS_KEY")

	if len(awsId) > 0 && len(awsKey) >0 {
		awsConfig.Credentials = credentials.NewStaticCredentials(awsId, awsKey,"")
	}

	sess, credentialsErr := session.NewSession(awsConfig)
	if credentialsErr != nil {
		msg := fmt.Sprintf("Error occurred while creating credentials %v\n", credentialsErr)
		return "", errors.New(msg)
	}

	svc := sns.New(sess)
	afterTaxes, afterTaxesErr := estimatedIncome(totalTime)
	if afterTaxesErr != nil {
		return "", afterTaxesErr
	}

	potentialIncome, potentialIncomeErr := estimatedIncome(totalTime + hoursLeft)
	if potentialIncomeErr != nil {
		return "", potentialIncomeErr
	}

	p := message.NewPrinter(language.English)
	msgPtr := p.Sprintf("Total Hours: %.2f\nEstimated Pay After Taxes %.2f PLN\nPotential Income: %.2f\n",
		totalTime, afterTaxes, potentialIncome)
	fmt.Print(msgPtr)

	topicPtr := os.Getenv("SNS_TOPIC_ARN")
	result, publishErr := svc.Publish(&sns.PublishInput{
		Message:  &msgPtr,
		TopicArn: &topicPtr,
	})
	if publishErr != nil {
		msg := fmt.Sprintf("Error occurred while sending sns %v\n", publishErr)
		return "", errors.New(msg)
	}

	fmt.Println(*result.MessageId)

	return msgPtr, nil
}

func estimatedIncome(totalTime float64) (float64, error) {
	if totalTime == 0.00 {
		return 0.00, nil
	}

	hourlyRate, hourlyConvertErr := strconv.ParseFloat(os.Getenv("HOURLY_RATE"), 64)
	if hourlyConvertErr != nil {
		msg := fmt.Sprintf("Error occurred while converting HOURLY_RATE %v\n", hourlyConvertErr)
		return 0.0, errors.New(msg)
	}
	monthlyGross := convertToUsdFromPLN(totalTime * hourlyRate)

	estimatedMonthlyFixedTaxes, convertErr := strconv.ParseFloat(os.Getenv("ESTIMATED_MONTHLY_FIXED_TAXES"), 64)
	if convertErr != nil {
		msg := fmt.Sprintf("Error occurred while converting ESTIMATED_MONTHLY_FIXED_TAXES %v\n", convertErr)
		return 0.0, errors.New(msg)
	}

	estimatedTaxRate, taxConvertErr := strconv.ParseFloat(os.Getenv("ESTIMATED_TAX_RATE"), 64)
	if taxConvertErr != nil {
		msg := fmt.Sprintf("Error occurred while converting ESTIMATED_TAX_RATE %v\n", taxConvertErr)
		return 0.0, errors.New(msg)
	}

	taxableIncome := monthlyGross - estimatedMonthlyFixedTaxes
	incomeTax := math.Max(0, taxableIncome * estimatedTaxRate)

	afterTaxes := taxableIncome - incomeTax

	return afterTaxes, nil
}

func convertToUsdFromPLN(pln float64) float64 {
	fc := forex.NewClient()

	params := map[string]string{"from": "usd", "to": "pln", "amount": fmt.Sprintf("%f", pln)}
	conversion, err := fc.Convert(params)
	if err != nil {
		log.Println(err)
	}

	return conversion.Result
}
