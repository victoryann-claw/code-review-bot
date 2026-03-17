const OpenAI = require('openai');

// Initialize OpenAI client
const openai = new OpenAI({
  apiKey: process.env.OPENAI_API_KEY
});

/**
 * Analyze code using OpenAI
 * @param {string} diff - Pull request diff
 * @param {object} prDetails - Pull request details
 * @returns {Promise<Array>} - Array of review issues
 */
async function analyzeCode(diff, prDetails) {
  const systemPrompt = `You are an expert code reviewer. Analyze the following GitHub pull request diff and identify potential issues, bugs, security vulnerabilities, code quality problems, or suggestions for improvement.

For each issue found, respond with a JSON array of objects with these fields:
- type: "bug", "security", "performance", "style", "suggestion"
- severity: "high", "medium", "low"
- file: filename (if applicable)
- line: line number (if applicable)
- description: brief description of the issue
- suggestion: how to fix or improve

Respond ONLY with a valid JSON array. If no issues found, return an empty array [].`;

  const userPrompt = `Pull Request #${prDetails.number}: ${prDetails.title}
Author: ${prDetails.author}
Branch: ${prDetails.head} -> ${prDetails.base}

Diff:
${diff}`;

  try {
    const response = await openai.chat.completions.create({
      model: process.env.OPENAI_MODEL || 'gpt-4',
      messages: [
        { role: 'system', content: systemPrompt },
        { role: 'user', content: userPrompt }
      ],
      temperature: 0.3,
      max_tokens: 8000
    });

    const content = response.choices[0].message.content;

    // Parse JSON response
    try {
      // Extract JSON from potential markdown code blocks
      const jsonMatch = content.match(/\[[\s\S]*\]/);
      if (jsonMatch) {
        return JSON.parse(jsonMatch[0]);
      }
      return JSON.parse(content);
    } catch (parseError) {
      console.error('Failed to parse AI response as JSON:', content);
      return [];
    }

  } catch (error) {
    console.error('OpenAI API error:', error);
    throw error;
  }
}

module.exports = {
  analyzeCode
};
