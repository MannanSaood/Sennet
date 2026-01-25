import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import * as z from "zod";
import { Link, useNavigate } from "react-router-dom";
import { useAuth } from "@/context/AuthContext";
import { AuthLayout } from "@/components/auth/AuthLayout";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { useToast } from "@/hooks/useToast";
import { Mail, Lock, User, Terminal } from "lucide-react";

const registerSchema = z.object({
    name: z.string().min(2, "Name must be at least 2 characters"),
    email: z.string().email("Please enter a valid email address"),
    password: z.string().min(8, "Password must be at least 8 characters"),
    confirmPassword: z.string(),
}).refine((data) => data.password === data.confirmPassword, {
    message: "Passwords do not match",
    path: ["confirmPassword"],
});

type RegisterForm = z.infer<typeof registerSchema>;

export function RegisterPage() {
    const { register: registerUser } = useAuth();
    const navigate = useNavigate();
    const { toast } = useToast();
    const [isLoading, setIsLoading] = useState(false);

    const {
        register,
        handleSubmit,
        formState: { errors },
    } = useForm<RegisterForm>({
        resolver: zodResolver(registerSchema),
    });

    const onSubmit = async (data: RegisterForm) => {
        setIsLoading(true);
        try {
            await registerUser(data.email, data.password, data.name);
            toast({
                title: "Account created!",
                description: "Welcome to Sennet. You are now logged in.",
                variant: "success",
            });
            navigate("/dashboard");
        } catch (error) {
            toast({
                title: "Registration failed",
                description: "Could not create account. Try a different email.",
                variant: "destructive",
            });
        } finally {
            setIsLoading(false);
        }
    };

    return (
        <AuthLayout
            title="Create an account"
            description="Get started with Sennet today. No credit card required."
        >
            <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
                <Input
                    placeholder="John Doe"
                    label="Full Name"
                    icon={<User className="h-4 w-4" />}
                    error={errors.name?.message}
                    {...register("name")}
                />

                <Input
                    placeholder="name@example.com"
                    type="email"
                    label="Email"
                    icon={<Mail className="h-4 w-4" />}
                    error={errors.email?.message}
                    {...register("email")}
                />

                <Input
                    placeholder="Create a password"
                    type="password"
                    label="Password"
                    icon={<Lock className="h-4 w-4" />}
                    error={errors.password?.message}
                    {...register("password")}
                />

                <Input
                    placeholder="Confirm password"
                    type="password"
                    label="Confirm Password"
                    icon={<Lock className="h-4 w-4" />}
                    error={errors.confirmPassword?.message}
                    {...register("confirmPassword")}
                />

                <Button className="w-full gap-2" type="submit" isLoading={isLoading}>
                    <Terminal className="h-4 w-4" />
                    Create Account
                </Button>
            </form>

            <div className="text-center text-sm text-text-secondary mt-6">
                Already have an account?{" "}
                <Link to="/login" className="font-medium text-accent hover:underline">
                    Sign in
                </Link>
            </div>
        </AuthLayout>
    );
}
