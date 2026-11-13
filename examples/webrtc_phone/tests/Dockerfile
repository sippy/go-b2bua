FROM mcr.microsoft.com/playwright:v1.56.1-noble
WORKDIR /tests
RUN npm install --save-exact playwright@1.56.1
COPY examples/webrtc_phone/tests/*.cjs ./
CMD ["node", "browser.cjs"]
