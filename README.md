# Automated harvest notifications via SMS/Email

[![CircleCI](https://circleci.com/gh/scottfrasso/automated-harvest/tree/main.svg?style=shield&circle-token=46802aecae116dffd1397404907163b43a28b1f3)](https://circleci.com/gh/scottfrasso/automated-harvest/tree/main)

This is a Go function I wrote that runs in AWS Lambda daily to send me an SNS and Email on
the possible income I could be making versus what I've logged in Harvest this month. This
is something I wrote early on as a contractor to motivate me to log more hours on an hourly
contract. I figured I'd share the code for it on github for fun.