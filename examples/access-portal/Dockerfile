FROM python:3.14-slim

WORKDIR /app

COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt && \
    addgroup --system quorum && adduser --system --group quorum

COPY . .

USER quorum

EXPOSE 3003

CMD ["python", "app.py"]
