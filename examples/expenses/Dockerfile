FROM node:25-alpine

WORKDIR /app
COPY package.json ./
RUN npm install --production

COPY . .

USER node

EXPOSE 3002

CMD ["node", "server.js"]
