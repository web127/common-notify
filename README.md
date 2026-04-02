# common-notify

A notification library for managing and sending common notifications across your applications.

## Features

- Simple and intuitive API for sending notifications
- Support for multiple notification types
- Easy integration into existing projects
- Configurable notification handling

## Installation

```bash
npm install common-notify
```

## Usage

```javascript
const { Notify } = require('common-notify');

// Create a new notifier instance
const notifier = new Notify();

// Send a notification
notifier.send({
  type: 'info',
  message: 'This is an info notification',
  duration: 3000
});
```

## Documentation

For more detailed documentation and examples, please refer to the [documentation](./docs) folder.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is open source and available under the MIT License.