<?php

use Illuminate\Support\Facades\Route;

Route::redirect('/', '/app/', 307);
Route::get('/app/{path?}', static fn () => response(file_get_contents(public_path('app/index.html')))->header('Content-Type', 'text/html; charset=utf-8'))->where('path', '.*');
Route::get('/admin/{path?}', static fn () => response(file_get_contents(public_path('admin/index.html')))->header('Content-Type', 'text/html; charset=utf-8'))->where('path', '.*');
