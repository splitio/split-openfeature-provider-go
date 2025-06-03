# Split OpenFeature Provider for Go
[![Twitter Follow](https://img.shields.io/twitter/follow/splitsoftware.svg?style=social&label=Follow&maxAge=1529000)](https://twitter.com/intent/follow?screen_name=harnessio)

## Overview
This Provider is designed to allow the use of OpenFeature with Split (Acquired by Harness), the platform for controlled rollouts, serving features to your users via the Split feature flag to manage your complete customer experience.

## Compatibility
This SDK is compatible with Go 1.23 and higher.

## Getting started
Below is a simple example that describes the instantiation of the Split Provider. Please see the [OpenFeature Documentation](https://docs.openfeature.dev/docs/reference/concepts/evaluation-api) for details on how to use the OpenFeature SDK.

```go
import (
    "github.com/open-feature/go-sdk/pkg/openfeature"
    splitProvider "github.com/splitio/split-openfeature-provider-go"
)

provider, err := splitProvider.NewProviderSimple("YOUR_SDK_TYPE_API_KEY")
if err != nil {
    // Provider creation error
}
openfeature.SetProvider(provider)

```

If you are more familiar with Split or want access to other initialization options, you can provide a `SplitClient` to the constructor. See the [Split Go SDK Documentation](https://help.split.io/hc/en-us/articles/360020093652-Go-SDK#initialization) for more information.
```go
import (
    "github.com/open-feature/go-sdk/pkg/openfeature"
    "github.com/splitio/go-client/v6/splitio/client"
    "github.com/splitio/go-client/v6/splitio/conf"
    splitProvider "github.com/splitio/split-openfeature-provider-go"
)

cfg := conf.Default()
factory, err := client.NewSplitFactory("YOUR_SDK_TYPE_API_KEY", cfg)
if err != nil {
    // SDK initialization error
}

splitClient := factory.Client()

err = splitClient.BlockUntilReady(10)
if err != nil {
    // SDK timeout error
}

provider, err := splitProvider.NewProvider(*splitClient)
if err != nil {
    // Provider creation error
}
openfeature.SetProvider(provider)
```

## Use of OpenFeature with Split
After the initial setup you can use OpenFeature according to their [documentation](https://docs.openfeature.dev/docs/reference/concepts/evaluation-api/).

One important note is that the Split Provider **requires a targeting key** to be set. Often times this should be set when evaluating the value of a flag by [setting an EvaluationContext](https://docs.openfeature.dev/docs/reference/concepts/evaluation-context) which contains the targeting key. An example flag evaluation is
```go
client := openfeature.NewClient("CLIENT_NAME");

evaluationContext := openfeature.NewEvaluationContext("TARGETING_KEY", nil)
boolValue := client.BooleanValue(nil, "boolFlag", false, evaluationContext)
```
If the same targeting key is used repeatedly, the evaluation context may be set at the client level 
```go
evaluationContext := openfeature.NewEvaluationContext("TARGETING_KEY", nil)
client.SetEvaluationContext(context)
```
or at the OpenFeatureAPI level 
```go
evaluationContext := openfeature.NewEvaluationContext("TARGETING_KEY", nil)
openfeature.SetEvaluationContext(context)
````
If the context was set at the client or api level, it is not required to provide it during flag evaluation.

## Submitting issues
 
The Split team monitors all issues submitted to this [issue tracker](https://github.com/splitio/split-openfeature-provider-go/issues). We encourage you to use this issue tracker to submit any bug reports, feedback, and feature enhancements. We'll do our best to respond in a timely manner.

## Contributing
Please see [Contributors Guide](CONTRIBUTORS-GUIDE.md) to find all you need to submit a Pull Request (PR).

## License
Licensed under the Apache License, Version 2.0. See: [Apache License](http://www.apache.org/licenses/).

## About Split
 
Split, now a part of Harness, is a leading Feature Delivery Platform for engineering teams that want to confidently deploy features as fast as they can develop them. Split’s fine-grained management, real-time monitoring, and data-driven experimentation ensure that new features will improve the customer experience without breaking or degrading performance. Companies like Twilio, Salesforce, GoDaddy and WePay have trusted Split to power their feature delivery.
 
To learn more about Harness Feature Management and Experimentation (formerly Split), visit the [Harness website](https://www.harness.io/products/feature-management-experimentation) or contact [Harness Sales](https://harness.io/contact/sales).
 
Harness  has built and maintains SDKs for:
 
* Java [Github](https://github.com/splitio/java-client) [Docs](https://developer.harness.io/docs/feature-management-experimentation/sdks-and-infrastructure/server-side-sdks/java-sdk)
* Javascript [Github](https://github.com/splitio/javascript-client) [Docs]()
* Node [Github](https://github.com/splitio/javascript-client) [Docs](https://developer.harness.io/docs/feature-management-experimentation/sdks-and-infrastructure/server-side-sdks/nodejs-sdk)
* .NET [Github](https://github.com/splitio/dotnet-client) [Docs](https://developer.harness.io/docs/feature-management-experimentation/sdks-and-infrastructure/server-side-sdks/net-sdk)
* Ruby [Github](https://github.com/splitio/ruby-client) [Docs](https://developer.harness.io/docs/feature-management-experimentation/sdks-and-infrastructure/server-side-sdks/ruby-sdk)
* PHP [Github](https://github.com/splitio/php-client) [Docs](https://developer.harness.io/docs/feature-management-experimentation/sdks-and-infrastructure/server-side-sdks/php-sdk)
* Python [Github](https://github.com/splitio/python-client) [Docs](https://developer.harness.io/docs/feature-management-experimentation/sdks-and-infrastructure/server-side-sdks/python-sdk)
* GO [Github](https://github.com/splitio/go-client) [Docs](https://help.split.io/hc/en-us/articles/360020093652-Go-SDK)
* Android [Github](https://github.com/splitio/android-client) [Docs](https://developer.harness.io/docs/feature-management-experimentation/sdks-and-infrastructure/client-side-sdks/android-sdk)
* iOS [Github](https://github.com/splitio/ios-client) [Docs](https://developer.harness.io/docs/feature-management-experimentation/sdks-and-infrastructure/client-side-sdks/ios-sdk)
 
For a comprehensive list of open source projects visit our [Github page](https://github.com/splitio?utf8=%E2%9C%93&query=%20only%3Apublic%20).
 
**Learn more about Harness:**

Visit [harness.io](https://www.harness.io) for an overview of Harness, or visit our documentation at [developer.harness.io/docs](https://developer.harness.io/docs) for more detailed information.
