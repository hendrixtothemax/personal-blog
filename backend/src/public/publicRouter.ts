// src/routes/publicRouter.ts
import { Router, Request, Response } from 'express';
import path from 'path';

const router = Router();

// Example: GET /public/info
router.get('/info', (req: Request, res: Response) => {
  res.send('This is from the public router!');
});

// Example: serve a file
router.get('/html', (req: Request, res: Response) => {
  res.sendFile(path.join(__dirname, '../../public/index.html'));
});

export default router;
