<?php

namespace Database\Factories;

use App\Models\User;
use Illuminate\Database\Eloquent\Factories\Factory;

/**
 * @extends Factory<User>
 */
class UserFactory extends Factory
{
    /**
     * Define the model's default state.
     *
     * @return array<string, mixed>
     */
    public function definition(): array
    {
        return [
            'full_name' => fake()->firstName().' '.fake()->lastName(),
            'phone' => '+7999'.fake()->unique()->numerify('#######'),
            'phone_verified' => false,
            'gender' => 'male',
            'age' => 25,
        ];
    }
}
