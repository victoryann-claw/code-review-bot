const { Octokit } = require('octokit');

// Initialize Octokit with GitHub token
const octokit = new Octokit({
  auth: process.env.GITHUB_TOKEN
});

/**
 * Get pull request details
 * @param {string} owner - Repository owner
 * @param {string} repo - Repository name
 * @param {number} prNumber - Pull request number
 * @returns {Promise<object>} - PR details
 */
async function getPullRequestDetails(owner, repo, prNumber) {
  const { data } = await octokit.rest.pulls.get({
    owner,
    repo,
    pull_number: prNumber
  });

  return {
    number: data.number,
    title: data.title,
    body: data.body,
    head: data.head.ref,
    base: data.base.ref,
    author: data.user.login,
    url: data.html_url
  };
}

/**
 * Get pull request diff
 * @param {string} owner - Repository owner
 * @param {string} repo - Repository name
 * @param {number} prNumber - Pull request number
 * @returns {Promise<string>} - PR diff
 */
async function getPullRequestDiff(owner, repo, prNumber) {
  const { data } = await octokit.rest.pulls.get({
    owner,
    repo,
    pull_number: prNumber,
    accept: 'application/vnd.github.v3.diff'
  });

  return data;
}

/**
 * Create a review comment on a pull request
 * @param {string} owner - Repository owner
 * @param {string} repo - Repository name
 * @param {number} prNumber - Pull request number
 * @param {string} body - Comment body (markdown)
 * @returns {Promise<object>} - Created comment
 */
async function createReviewComment(owner, repo, prNumber, body) {
  const { data } = await octokit.rest.issues.createComment({
    owner,
    repo,
    issue_number: prNumber,
    body
  });

  return data;
}

/**
 * Create a review with inline comments
 * @param {string} owner - Repository owner
 * @param {string} repo - Repository name
 * @param {number} prNumber - Pull request number
 * @param {string} body - Review body
 * @param {Array} comments - Array of inline comments
 * @returns {Promise<object>} - Created review
 */
async function createPullRequestReview(owner, repo, prNumber, body, comments) {
  const { data } = await octokit.rest.pulls.createReview({
    owner,
    repo,
    pull_number: prNumber,
    body,
    event: 'COMMENT',
    comments
  });

  return data;
}

module.exports = {
  getPullRequestDetails,
  getPullRequestDiff,
  createReviewComment,
  createPullRequestReview
};
