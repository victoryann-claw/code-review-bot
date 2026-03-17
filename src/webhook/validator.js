const crypto = require('crypto');

/**
 * Validates GitHub webhook signature
 * @param {string} signature - X-Hub-Signature-256 header value
 * @param {string} payload - Raw request body
 * @param {string} secret - Webhook secret
 * @returns {boolean} - Whether signature is valid
 */
function validateSignature(signature, payload, secret) {
  if (!signature || !payload || !secret) {
    return false;
  }

  const hmac = crypto.createHmac('sha256', secret);
  const digest = 'sha256=' + hmac.update(payload).digest('hex');

  try {
    return crypto.timingSafeEqual(
      Buffer.from(signature),
      Buffer.from(digest)
    );
  } catch (error) {
    return false;
  }
}

/**
 * Extracts signature from headers
 * @param {object} headers - Request headers
 * @returns {string|null} - Signature value
 */
function getSignature(headers) {
  return headers['x-hub-signature-256'] || null;
}

module.exports = {
  validateSignature,
  getSignature
};
