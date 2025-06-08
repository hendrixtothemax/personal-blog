import express from 'express';
import publicRouter from './public/publicRouter'

const app = express();
const port = 3000;

app.use('/public', publicRouter)

app.get('/', (req, res) => {
    res.send('Hello from Express with TypeScript!');
});

app.listen(port, () => {
    console.log(`Server running at http://localhost:${port}`);
});

