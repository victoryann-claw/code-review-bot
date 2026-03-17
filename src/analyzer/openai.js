const OpenAI = require('openai');

/**
 * LLM Provider Factory
 * Supports: OpenAI, MiniMax
 */
class LLMProvider {
  constructor() {
    this.provider = process.env.LLM_PROVIDER || 'openai';
    
    if (this.provider === 'minimax') {
      this.minimax = new OpenAI({
        apiKey: process.env.MINIMAX_API_KEY,
        baseURL: 'https://api.minimax.chat/v1'
      });
    } else {
      // Default to OpenAI
      this.openai = new OpenAI({
        apiKey: process.env.OPENAI_API_KEY
      });
    }
  }

  async chatCompletion(messages, options = {}) {
    const defaultOptions = {
      temperature: 0.3,
      max_tokens: 8000
    };

    const opts = { ...defaultOptions, ...options };

    if (this.provider === 'minimax') {
      // MiniMax uses different model format: MiniMax-Text-01
      return await this.minimax.chat.completions.create({
        model: process.env.MINIMAX_MODEL || 'MiniMax-Text-01',
        messages,
        ...opts
      });
    } else {
      return await this.openai.chat.completions.create({
        model: process.env.OPENAI_MODEL || 'gpt-4',
        messages,
        ...opts
      });
    }
  }
}

const llm = new LLMProvider();

/**
 * Analyze code using LLM (OpenAI or MiniMax)
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
    const response = await llm.chatCompletion([
      { role: 'system', content: systemPrompt },
      { role: 'user', content: userPrompt }
    ]);

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
    console.error('LLM API error:', error);
    throw error;
  }
}

module.exports = {
  analyzeCode
};
