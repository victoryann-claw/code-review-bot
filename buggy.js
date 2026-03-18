// Buggy code
const userInput = require('userInput');

function process() {
  eval(userInput);
  var x = 1;
}

process();
// using var instead of let/const
