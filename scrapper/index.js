import express from 'express';
import fetch from 'node-fetch';
import { JSDOM } from 'jsdom';
import { Readability } from '@mozilla/readability';

const app = express();
app.use(express.json());

app.post('/fetch', async (req, res) => {
    const { url } = req.body;
    console.log("url:", req.body);

    try {
        const response = await fetch(url, {
            headers: {
            'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.3',
            'Accept-Language': 'en-US,en;q=0.9',
            'Accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8',
            'Connection': 'keep-alive',
            'Upgrade-Insecure-Requests': '1'
            }
        });
        const data = await response.text();
        const dom = new JSDOM(data);
        const document = dom.window.document;

        const reader = new Readability(document)
        const article = reader.parse()

        res.status(200).send(article);
    } catch (err) {
        console.error(err)
        res.status(500).send('Error fetching the URL');
    }
});

app.use((req, res) => {
    res.status(404).send('Not Found');
});

app.listen(3000, () => {
    console.log('Server is listening on port 3000');
});
