require('dotenv').config();
const express = require('express');
const webhookHandler = require('./src/webhook/handler');

const app = express();
const PORT = process.env.PORT || 3000;

// Middleware to parse JSON
app.use(express.json({
  verify: (req, res, buf) => {
    req.rawBody = buf.toString();
  }
}));

// Health check endpoint
app.get('/', (req, res) => {
  res.json({ status: 'ok', message: 'CodeReviewBot is running' });
});

// GitHub Webhook endpoint
app.post('/webhook', webhookHandler);

// Start server
app.listen(PORT, () => {
  console.log(`CodeReviewBot is listening on port ${PORT}`);
  console.log(`Configure your GitHub webhook URL to: http://<your-server>/webhook`);
});

module.exports = app;
