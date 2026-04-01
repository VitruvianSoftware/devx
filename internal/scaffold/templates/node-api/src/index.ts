import express from 'express'
import dotenv from 'dotenv'

dotenv.config()

const app = express()
const port = process.env.PORT ?? 8080

app.use(express.json())

// Health check
app.get('/healthz', (_req, res) => {
  res.json({ status: 'ok', timestamp: new Date().toISOString() })
})

// API v1
const v1 = express.Router()
app.use('/api/v1', v1)

// Add your routes:
// v1.get('/items', listItems)
// v1.post('/items', createItem)

app.listen(port, () => {
  console.log(`Server running on http://localhost:${port}`)
})

export default app
