const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');

// Run the actual model-loading function with only its DOM/network boundaries stubbed.
const html = fs.readFileSync(`${__dirname}/templates/index.html`, 'utf8');
const source = html.slice(html.indexOf('async function loadModels()'), html.indexOf("modelSelector.addEventListener('change'"));

async function load(current, postOK = true, models = [{ provider: 'openai', id: 'gpt-4o' }]) {
    const options = [];
    const selector = { disabled: true, innerHTML: '', appendChild() {}, querySelector: () => options[0] };
    const posts = [];
    const document = { createElement: tag => {
        const element = { appendChild() {} };
        if (tag === 'option') options.push(element);
        return element;
    } };
    const fetch = async (url, opts) => {
        if (opts?.method === 'POST') {
            posts.push(JSON.parse(opts.body));
            return { ok: postOK, text: async () => 'failed' };
        }
        return { json: async () => url === '/api/models'
            ? { models }
            : current };
    };
    await vm.runInNewContext(`${source}\nloadModels()`, {
        fetch, document, modelSelector: selector, providerLabels: {}, console: { error() {} }
    });
    return { posts, selector, options };
}

test('recovered model list applies the first model to the agent', async () => {
    const { posts, selector } = await load({ provider: 'openai', model: '' });
    assert.deepEqual(posts, [{ provider: 'openai', model: 'gpt-4o' }]);
    assert.equal(selector.disabled, false);
});

test('loading preserves an already selected model', async () => {
    const { posts } = await load({ provider: 'openai', model: 'gpt-4o' });
    assert.deepEqual(posts, []);
});

for (const current of [
    { provider: 'ollama', model: 'local-model' },
    { provider: 'openai', model: 'temporarily-unavailable-model' }
]) {
    test(`discovery failure preserves ${current.provider}/${current.model}`, async () => {
        const { posts, options } = await load(current);
        assert.deepEqual(posts, []);
        const selected = options.find(option => option.selected);
        assert.ok(selected?.disabled, 'show the unavailable selection instead of another model');
        assert.match(selected.textContent, new RegExp(current.model));
    });
}

test('an empty local selection never falls back to a cloud provider', async () => {
    const { posts } = await load({ provider: 'ollama', model: '' });
    assert.deepEqual(posts, []);
});

test('initial selection uses the current provider even when another provider is first', async () => {
    const { posts } = await load({ provider: 'openrouter', model: '' }, true, [
        { provider: 'openai', id: 'gpt-4o' },
        { provider: 'openrouter', id: 'vendor/model' }
    ]);
    assert.deepEqual(posts, [{ provider: 'openrouter', model: 'vendor/model' }]);
});

test('failed automatic selection is reported instead of pretending it succeeded', async () => {
    const { selector } = await load({ provider: 'openai', model: '' }, false);
    assert.match(selector.innerHTML, /Failed to load models/);
});
