const { validateSignature, getSignature } = require('./validator');
const { getPullRequestDiff, getPullRequestDetails } = require('../github/client');
const { analyzeCode } = require('../analyzer/openai');
const { formatReviewComment } = require('../comment/formatter');
const { createReviewComment } = require('../github/client');

/**
 * Handle GitHub webhook events
 */
async function handleWebhook(req, res) {
  try {
    const signature = getSignature(req.headers);
    const secret = process.env.GITHUB_WEBHOOK_SECRET;

    // Validate signature if secret is configured
    if (secret && !validateSignature(signature, req.rawBody, secret)) {
      console.error('Invalid webhook signature');
      return res.status(401).json({ error: 'Invalid signature' });
    }

    const event = req.headers['x-github-event'];
    const action = req.body.action;

    console.log(`Received event: ${event}, action: ${action}`);

    // Only process pull request events
    if (event !== 'pull_request') {
      return res.status(200).json({ message: 'Event ignored' });
    }

    // Only process when PR is opened or synchronize (new commits)
    if (!['opened', 'synchronize'].includes(action)) {
      return res.status(200).json({ message: 'Action ignored' });
    }

    const pr = req.body.pull_request;
    const repo = req.body.repository;
    const owner = repo.owner.login;
    const repoName = repo.name;
    const prNumber = pr.number;

    console.log(`Processing PR #${prNumber} from ${owner}/${repoName}`);

    // Get PR details and diff
    const [prDetails, diff] = await Promise.all([
      getPullRequestDetails(owner, repoName, prNumber),
      getPullRequestDiff(owner, repoName, prNumber)
    ]);

    if (!diff || diff.length === 0) {
      console.log('No diff found, skipping review');
      return res.status(200).json({ message: 'No changes to review' });
    }

    // Analyze code with OpenAI
    console.log('Analyzing code with OpenAI...');
    const analysis = await analyzeCode(diff, prDetails);

    if (!analysis || analysis.length === 0) {
      console.log('No issues found by AI');
      return res.status(200).json({ message: 'No issues found' });
    }

    // Format and post review comments
    console.log(`Posting ${analysis.length} review comments...`);
    const comment = formatReviewComment(analysis);

    await createReviewComment(owner, repoName, prNumber, comment);

    console.log('Review completed successfully');
    res.status(200).json({ success: true, comments: analysis.length });

  } catch (error) {
    console.error('Error processing webhook:', error);
    res.status(500).json({ error: 'Internal server error' });
  }
}

module.exports = handleWebhook;
