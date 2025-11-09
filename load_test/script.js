import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '10s', target: 10 },   // быстрый разгон до 50 RPS
    { duration: '30s', target: 10 },  // затем до 100 RPS
    { duration: '20s', target: 0 },    // завершение
  ],
};

let userCounter = 1;

export default function () {
  const userName = `TestUser_${userCounter}_${Date.now()}`;
  const userEmail = `test${userCounter}_${Date.now()}@test.com`;

  const payload = {
    name: userName,
    email: userEmail,
  };

  const params = {
    headers: {
      'Content-Type': 'application/x-www-form-urlencoded',
    },
  };

  const response = http.post('http://localhost:8080/users/create', payload, params);

  check(response, {
    'user created successfully': (r) => r.status === 201,
  });

  userCounter++;
  sleep(0.1); // ~10 RPS на виртуального пользователя
}
