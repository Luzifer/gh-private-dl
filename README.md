# Luzifer / GH-PrivateDL

GH-PrivateDL is a [Sparta](http://gosparta.io/) project to create and deploy an [AWS Lambda](https://aws.amazon.com/lambda/) function which allows to `curl` a Github release asset from a private project without having to build a script around it and parse the API responses within that script.

## Why?

- **Why this project?** - Simple: I do have the need to put binaries hosted in private Github projects onto machines and I dislike fiddling around with scripts, `jq` and whatever to get the private download URLs. I just want to do a plain old `curl ...` call and get my asset.
- **Why Lambda?** - People are talking about serverless and Lambda, I stumbled upon the Sparta framework, I wanted to try it myself. It's that simple. It would have been way easier to use a Go binary doing the same inside a Docker container but hey where's the fun?

## How to use this?

- First step: Get yourself an AWS account and some IAM credentials being able to do stuff...
- Create a S3 bucket where all the intermediate stuff can be dropped before deploying
- Get this repo cloned onto your disk and build the Go binary or just fetch a precompiled [release](https://github.com/Luzifer/gh-private-dl/releases/latest)
- Execute the deployment:  
`./gh-private-dl provision -s <your bucket>`
- Get a lot of coffee. The initial deployment will take like forever.

After you've deployed your own copy of this you will see some `https://....execute-api.us-east-1.amazonaws.com` URL inside your terminal. That's the sign everything is up and you can use it. Remember that URL.

Lets say if you point your browser to the release page of your repository you copy the download URL of your asset:  
`https://github.com/Luzifer/vault2env/releases/download/v0.6.1/vault2env_linux_amd64`

- If you try to `curl` that one (this example indeed works because it's a public repo) you will get an `HTTP 404`.
- If you put your access token into your curl command (`curl -u auth:mysecrettoken ...`) you will get an `HTTP 404`. (Hey Github, fix that please!)
- Now lets transform the URL you've already got into one pointing to your copy of GH-PrivateDL:  
`https://....execute-api.us-east-1.amazonaws.com/prod/Luzifer/vault2env/releases/download/v0.6.1/vault2env_linux_amd64`
- Try `curl`ing that with your access token and you will get a redirect...

Now try this:

```
# curl -L -o /tmp/vault2env -u auth:mysecrettoken \
    https://....execute-api.us-east-1.amazonaws.com/prod/Luzifer/vault2env/releases/download/v0.6.1/vault2env_linux_amd64
```

You should have a binary sitting in `/tmp/vault2env` and everything is fine. (Okay as I said, it's a public project which makes the success not that huge but now you can imagine your own URLs with your own private projects and **then** you will be happy...)

### Updating

To update the lambda function just repeat the deployment process. It will take way less time and after it finished you will have a new version at the same URL.

## How to use this if you don't want to run it with AWS Lambda?

The binary also contains a `run` command which will start the service locally. In that case you can use it like any other Go webserver.

Also there is a Docker container available which internally uses the `run` command:

```
# docker run --rm -ti -p 3000:3000 quay.io/luzifer/privatehub
```
