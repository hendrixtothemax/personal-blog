// src/routes/publicRouter.ts
import { Router, Request, Response } from 'express';
import path from 'path';

const router = Router();

// Example: GET /public/info
router.get('/info', (req: Request, res: Response) => {
    res.send('This is from the public router!');
});

router.get('/css/:cssFileName', (req: Request, res: Response) => {
  const cssFileName = req.params.cssFileName;

  // Sanitize and resolve the full path
  const filePath = path.resolve(__dirname, `../../../frontend/css/${cssFileName}`);

  res.sendFile(filePath, (err) => {
    if (err) {
      console.error(err);
      res.status(404).send('CSS file not found');
    }
  });
});

router.get('/js/:jsFileName', (req: Request, res: Response) => {
  const jsFileName = req.params.jsFileName;

  // Sanitize and resolve the full path
  const filePath = path.resolve(__dirname, `../../../frontend/js/${jsFileName}`);

  res.sendFile(filePath, (err) => {
    if (err) {
      console.error(err);
      res.status(404).send('CSS file not found');
    }
  });
});

export default router;
