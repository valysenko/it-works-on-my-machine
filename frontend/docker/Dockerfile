FROM node:24-alpine

WORKDIR /app

COPY package*.json ./

RUN npm install

COPY public/ ./public/
COPY src/ ./src/

RUN npm run build

CMD ["npm", "start"]